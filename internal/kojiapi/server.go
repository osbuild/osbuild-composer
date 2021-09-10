// Package kojiapi provides a REST API to build and push images to Koji
package kojiapi

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/kojiapi/api"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

// Server represents the state of the koji Server
type Server struct {
	logger      *log.Logger
	workers     *worker.Server
	rpmMetadata rpmmd.RPMMD
	distros     *distroregistry.Registry
}

// NewServer creates a new koji server
func NewServer(logger *log.Logger, workers *worker.Server, rpmMetadata rpmmd.RPMMD, distros *distroregistry.Registry) *Server {
	s := &Server{
		logger:      logger,
		workers:     workers,
		rpmMetadata: rpmMetadata,
		distros:     distros,
	}

	return s
}

// Create an http.Handler() for this server, that provides the koji API at the
// given path.
func (s *Server) Handler(path string) http.Handler {
	e := echo.New()
	e.Binder = binder{}
	e.StdLogger = s.logger

	// log errors returned from handlers
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		log.Println(c.Path(), c.QueryParams().Encode(), err.Error())
		e.DefaultHTTPErrorHandler(err, c)
	}

	api.RegisterHandlers(e.Group(path), &apiHandlers{s})

	return e
}

// apiHandlers implements api.ServerInterface - the http api route handlers
// generated from api/openapi.yml. This is a separate object, because these
// handlers should not be exposed on the `Server` object.
type apiHandlers struct {
	server *Server
}

// PostCompose handles a new /compose POST request
func (h *apiHandlers) PostCompose(ctx echo.Context) error {
	var request api.ComposeRequest
	err := ctx.Bind(&request)
	if err != nil {
		return err
	}

	d := h.server.distros.GetDistro(request.Distribution)
	if d == nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unsupported distribution: %s", request.Distribution))
	}

	type imageRequest struct {
		manifest      distro.Manifest
		arch          string
		filename      string
		exports       []string
		pipelineNames *worker.PipelineNames
	}

	imageRequests := make([]imageRequest, len(request.ImageRequests))
	kojiFilenames := make([]string, len(request.ImageRequests))
	kojiDirectory := "osbuild-composer-koji-" + uuid.New().String()

	// use the same seed for all images so we get the same IDs
	bigSeed, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		panic("cannot generate a manifest seed: " + err.Error())
	}
	manifestSeed := bigSeed.Int64()

	for i, ir := range request.ImageRequests {
		arch, err := d.GetArch(ir.Architecture)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unsupported architecture '%s' for distribution '%s'", ir.Architecture, request.Distribution))
		}
		imageType, err := arch.GetImageType(ir.ImageType)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unsupported image type '%s' for %s/%s", ir.ImageType, ir.Architecture, request.Distribution))
		}
		repositories := make([]rpmmd.RepoConfig, len(ir.Repositories))
		for j, repo := range ir.Repositories {
			repositories[j].BaseURL = repo.Baseurl
			if repo.Gpgkey != nil {
				repositories[j].GPGKey = *repo.Gpgkey
			}
		}
		bp := &blueprint.Blueprint{}
		err = bp.Initialize()
		if err != nil {
			panic("Could not initialize empty blueprint.")
		}

		packageSets := imageType.PackageSets(*bp)
		packageSpecSets := make(map[string][]rpmmd.PackageSpec)
		for name, packages := range packageSets {
			packageSpecs, _, err := h.server.rpmMetadata.Depsolve(packages, repositories, d.ModulePlatformID(), arch.Name(), d.Releasever())
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to depsolve base base packages for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err))
			}
			packageSpecSets[name] = packageSpecs
		}

		manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, repositories, packageSpecSets, manifestSeed)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadGateway, fmt.Sprintf("Failed to get manifest for for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err))
		}

		imageRequests[i].manifest = manifest
		imageRequests[i].arch = arch.Name()
		imageRequests[i].filename = imageType.Filename()
		imageRequests[i].exports = imageType.Exports()
		imageRequests[i].pipelineNames = &worker.PipelineNames{
			Build:   imageType.BuildPipelines(),
			Payload: imageType.PayloadPipelines(),
		}

		kojiFilenames[i] = fmt.Sprintf(
			"%s-%s-%s.%s%s",
			request.Name,
			request.Version,
			request.Release,
			ir.Architecture,
			splitExtension(imageType.Filename()),
		)
	}

	initID, err := h.server.workers.EnqueueKojiInit(&worker.KojiInitJob{
		Server:  request.Koji.Server,
		Name:    request.Name,
		Version: request.Version,
		Release: request.Release,
	})
	if err != nil {
		// This is a programming error.
		panic(err)
	}

	var buildIDs []uuid.UUID
	for i, ir := range imageRequests {
		id, err := h.server.workers.EnqueueOSBuildKoji(ir.arch, &worker.OSBuildKojiJob{
			Manifest:      ir.manifest,
			ImageName:     ir.filename,
			Exports:       ir.exports,
			PipelineNames: ir.pipelineNames,
			KojiServer:    request.Koji.Server,
			KojiDirectory: kojiDirectory,
			KojiFilename:  kojiFilenames[i],
		}, initID)
		if err != nil {
			// This is a programming error.
			panic(err)
		}
		buildIDs = append(buildIDs, id)
	}

	id, err := h.server.workers.EnqueueKojiFinalize(&worker.KojiFinalizeJob{
		Server:        request.Koji.Server,
		Name:          request.Name,
		Version:       request.Version,
		Release:       request.Release,
		KojiFilenames: kojiFilenames,
		KojiDirectory: kojiDirectory,
		TaskID:        uint64(request.Koji.TaskId),
		StartTime:     uint64(time.Now().Unix()),
	}, initID, buildIDs)
	if err != nil {
		// This is a programming error.
		panic(err)
	}

	// TODO: remove
	// For backwards compatibility we must only return once the
	// build ID is known. This logic should live in the client,
	// and `JobStatus()` should have a way to block until it
	// changes.
	var initResult worker.KojiInitJobResult
	for {
		status, _, err := h.server.workers.JobStatus(initID, &initResult)
		if err != nil {
			panic(err)
		}
		if !status.Finished.IsZero() || status.Canceled {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if initResult.KojiError != "" {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Could not initialize build with koji: %v", initResult.KojiError))
	}

	return ctx.JSON(http.StatusCreated, &api.ComposeResponse{
		Id:          id.String(),
		KojiBuildId: int(initResult.BuildID),
	})
}

// splitExtension returns the extension of the given file. If there's
// a multipart extension (e.g. file.tar.gz), it returns all parts (e.g.
// .tar.gz). If there's no extension in the input, it returns an empty
// string. If the filename starts with dot, the part before the second dot
// is not considered as an extension.
func splitExtension(filename string) string {
	filenameParts := strings.Split(filename, ".")

	if len(filenameParts) > 0 && filenameParts[0] == "" {
		filenameParts = filenameParts[1:]
	}

	if len(filenameParts) <= 1 {
		return ""
	}

	return "." + strings.Join(filenameParts[1:], ".")
}

func composeStatusFromJobStatus(js *worker.JobStatus, initResult *worker.KojiInitJobResult, buildResults []worker.OSBuildKojiJobResult, result *worker.KojiFinalizeJobResult) string {
	if js.Canceled {
		return "failure"
	}

	if js.Finished.IsZero() {
		return "pending"
	}

	if initResult.KojiError != "" {
		return "failure"
	}

	for _, buildResult := range buildResults {
		if buildResult.OSBuildOutput != nil && !buildResult.OSBuildOutput.Success {
			return "failure"
		}
		if buildResult.KojiError != "" {
			return "failure"
		}
	}

	if result.KojiError != "" {
		return "failure"
	}

	return "success"
}

func imageStatusFromJobStatus(js *worker.JobStatus, initResult *worker.KojiInitJobResult, buildResult *worker.OSBuildKojiJobResult) string {
	if js.Canceled {
		return "failure"
	}

	if initResult.KojiError != "" {
		return "failure"
	}

	if js.Started.IsZero() {
		return "pending"
	}

	if js.Finished.IsZero() {
		return "building"
	}

	if buildResult.OSBuildOutput != nil && buildResult.OSBuildOutput.Success && buildResult.KojiError == "" {
		return "success"
	}

	return "failure"
}

// GetComposeId handles a /compose/{id} GET request
func (h *apiHandlers) GetComposeId(ctx echo.Context, idstr string) error {
	id, err := uuid.Parse(idstr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter id: %s", err))
	}

	// Make sure id exists and matches a FinalizeJob
	if _, _, err := h.getFinalizeJob(id); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Job %s not found: %s", idstr, err))
	}

	var finalizeResult worker.KojiFinalizeJobResult
	finalizeStatus, deps, err := h.server.workers.JobStatus(id, &finalizeResult)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Job %s not found: %s", idstr, err))
	}

	// Make sure deps[0] matches a KojiInitJob
	if _, err := h.getInitJob(deps[0]); err != nil {
		panic(err)
	}
	var initResult worker.KojiInitJobResult
	_, _, err = h.server.workers.JobStatus(deps[0], &initResult)
	if err != nil {
		// this is a programming error
		panic(err)
	}

	var buildResults []worker.OSBuildKojiJobResult
	var imageStatuses []api.ImageStatus
	for i := 1; i < len(deps); i++ {
		// Make sure deps[i] matches an OSBuildKojiJob
		if _, _, err := h.getBuildJob(deps[i]); err != nil {
			panic(err)
		}
		var buildResult worker.OSBuildKojiJobResult
		jobStatus, _, err := h.server.workers.JobStatus(deps[i], &buildResult)
		if err != nil {
			// this is a programming error
			panic(err)
		}
		buildResults = append(buildResults, buildResult)
		imageStatuses = append(imageStatuses, api.ImageStatus{
			Status: imageStatusFromJobStatus(jobStatus, &initResult, &buildResult),
		})
	}

	response := api.ComposeStatus{
		Status:        composeStatusFromJobStatus(finalizeStatus, &initResult, buildResults, &finalizeResult),
		ImageStatuses: imageStatuses,
	}
	buildID := int(initResult.BuildID)
	if buildID != 0 {
		response.KojiBuildId = &buildID
	}
	return ctx.JSON(http.StatusOK, response)
}

// GetStatus handles a /status GET request
func (h *apiHandlers) GetStatus(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, &api.Status{
		Status: "OK",
	})
}

// Get logs for a compose
func (h *apiHandlers) GetComposeIdLogs(ctx echo.Context, idstr string) error {
	id, err := uuid.Parse(idstr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter id: %s", err))
	}

	// Make sure id exists and matches a FinalizeJob
	if _, _, err := h.getFinalizeJob(id); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Job %s not found: %s", idstr, err))
	}

	var finalizeResult worker.KojiFinalizeJobResult
	_, deps, err := h.server.workers.JobStatus(id, &finalizeResult)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Job %s not found: %s", idstr, err))
	}

	// Make sure deps[0] matches a KojiInitJob
	if _, err := h.getInitJob(deps[0]); err != nil {
		panic(err)
	}

	var initResult worker.KojiInitJobResult
	_, _, err = h.server.workers.JobStatus(deps[0], &initResult)
	if err != nil {
		// This is a programming error.
		panic(err)
	}

	var buildResults []interface{}
	for i := 1; i < len(deps); i++ {
		// Make sure deps[i] matches an OSBuildKojiJob
		if _, _, err := h.getBuildJob(deps[i]); err != nil {
			panic(err)
		}
		var buildResult worker.OSBuildJobResult
		_, _, err = h.server.workers.JobStatus(deps[i], &buildResult)
		if err != nil {
			// This is a programming error.
			panic(err)
		}
		buildResults = append(buildResults, buildResult)
	}

	// Return the OSBuildJobResults as-is for now. The contents of ImageLogs
	// is not part of the API. It's meant for a human to be able to access
	// the logs, which just happen to be in JSON.
	response := api.ComposeLogs{
		KojiInitLogs:   initResult,
		KojiImportLogs: finalizeResult,
		ImageLogs:      buildResults,
	}

	return ctx.JSON(http.StatusOK, response)
}

// getFinalizeJob retrieves a KojiFinalizeJob and the IDs of its dependencies
// from the job queue given its ID.  It returns an error if the ID matches a
// job of a different type.
func (h *apiHandlers) getFinalizeJob(id uuid.UUID) (*worker.KojiFinalizeJob, []uuid.UUID, error) {
	job := new(worker.KojiFinalizeJob)
	jobType, _, deps, err := h.server.workers.Job(id, job)
	if err != nil {
		return nil, nil, err
	}
	expType := "koji-finalize"
	if jobType != expType {
		return nil, nil, fmt.Errorf("expected %q, found %q job instead", expType, jobType)
	}
	return job, deps, err
}

// getInitJob retrieves a KojiInitJob from the job queue given its ID.
func (h *apiHandlers) getInitJob(id uuid.UUID) (*worker.KojiInitJob, error) {
	// It returns an error if the ID matches a job of a different type.
	job := new(worker.KojiInitJob)
	jobType, _, _, err := h.server.workers.Job(id, job)
	if err != nil {
		return nil, err
	}
	expType := "koji-init"
	if jobType != expType {
		return nil, fmt.Errorf("expected %q, found %q job instead", expType, jobType)
	}
	return job, err
}

// getBuildJob retrieves a OSBuildKojiJob and the IDs of its dependencies from
// the job queue given its ID.  It returns an error if the ID matches a job of
// a different type.
func (h *apiHandlers) getBuildJob(id uuid.UUID) (*worker.OSBuildKojiJob, []uuid.UUID, error) {
	job := new(worker.OSBuildKojiJob)
	jobType, _, deps, err := h.server.workers.Job(id, job)
	if err != nil {
		return nil, nil, err
	}
	expType := "osbuild-koji"
	if !strings.HasPrefix(jobType, expType) { // Build jobs get automatic arch suffix: Check prefix
		return nil, nil, fmt.Errorf("expected %q, found %q job instead", expType, jobType)
	}
	return job, deps, nil
}

// GetComposeIdManifests returns the Manifests for a given Compose (one for each image).
func (h *apiHandlers) GetComposeIdManifests(ctx echo.Context, idstr string) error {
	id, err := uuid.Parse(idstr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid format for parameter id: %s", err))
	}

	_, deps, err := h.getFinalizeJob(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Job %s not found: %s", idstr, err))
	}

	manifests := make([]distro.Manifest, len(deps)-1)
	for i, id := range deps[1:] {
		buildJob, _, err := h.getBuildJob(id)
		if err != nil {
			// This is a programming error.
			panic(err)
		}
		manifests[i] = buildJob.Manifest
	}

	return ctx.JSON(http.StatusOK, manifests)
}

// A simple echo.Binder(), which only accepts application/json, but is more
// strict than echo's DefaultBinder. It does not handle binding query
// parameters either.
type binder struct{}

func (b binder) Bind(i interface{}, ctx echo.Context) error {
	request := ctx.Request()

	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		return echo.NewHTTPError(http.StatusUnsupportedMediaType, "request must be json-encoded")
	}

	err := json.NewDecoder(request.Body).Decode(i)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("cannot parse request body: %v", err))
	}

	return nil
}
