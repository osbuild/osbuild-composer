// Package kojiapi provides a REST API to build and push images to Koji
package kojiapi

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/kojiapi/api"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

// Server represents the state of the koji Server
type Server struct {
	logger      *log.Logger
	workers     *worker.Server
	rpmMetadata rpmmd.RPMMD
	distros     *distro.Registry
	kojiServers map[string]koji.GSSAPICredentials
}

// NewServer creates a new koji server
func NewServer(logger *log.Logger, workers *worker.Server, rpmMetadata rpmmd.RPMMD, distros *distro.Registry, kojiServers map[string]koji.GSSAPICredentials) *Server {
	s := &Server{
		logger:      logger,
		workers:     workers,
		rpmMetadata: rpmMetadata,
		distros:     distros,
		kojiServers: kojiServers,
	}

	return s
}

// Create an http.Handler() for this server, that provides the koji API at the
// given path.
func (s *Server) Handler(path string) http.Handler {
	e := echo.New()
	e.Binder = binder{}
	e.StdLogger = s.logger

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

	kojiServer, err := url.Parse(request.Koji.Server)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid Koji server: %s", request.Koji.Server))
	}
	creds, exists := h.server.kojiServers[kojiServer.Hostname()]
	if !exists {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Koji server has not been configured: %s", kojiServer.Hostname()))
	}

	type imageRequest struct {
		manifest     distro.Manifest
		arch         string
		filename     string
		kojiFilename string
	}
	imageRequests := make([]imageRequest, len(request.ImageRequests))

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
		packageSpecs, _ := imageType.Packages(*bp)
		packages, _, err := h.server.rpmMetadata.Depsolve(packageSpecs, nil, repositories, d.ModulePlatformID(), arch.Name())
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to depsolve base base packages for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err))
		}
		buildPackageSpecs := imageType.BuildPackages()
		buildPackages, _, err := h.server.rpmMetadata.Depsolve(buildPackageSpecs, nil, repositories, d.ModulePlatformID(), arch.Name())
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to depsolve build packages for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err))
		}

		manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, repositories, packages, buildPackages)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadGateway, fmt.Sprintf("Failed to get manifest for for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err))
		}

		imageRequests[i].manifest = manifest
		imageRequests[i].arch = arch.Name()
		imageRequests[i].filename = imageType.Filename()

		filename := fmt.Sprintf(
			"%s-%s-%s.%s%s",
			request.Name,
			request.Version,
			request.Release,
			ir.Architecture,
			splitExtension(imageType.Filename()),
		)

		imageRequests[i].kojiFilename = filename
	}

	var ir imageRequest
	if len(imageRequests) == 1 {
		// NOTE: the store currently does not support multi-image composes
		ir = imageRequests[0]
	} else {
		return echo.NewHTTPError(http.StatusBadRequest, "Only single-image composes are currently supported")
	}

	// Koji for some reason needs TLS renegotiation enabled.
	// Clone the default http transport and enable renegotiation.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		Renegotiation: tls.RenegotiateOnceAsClient,
	}

	k, err := koji.NewFromGSSAPI(request.Koji.Server, &creds, transport)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Could not log into Koji: %v", err))
	}

	defer func() {
		err := k.Logout()
		if err != nil {
			log.Printf("koji logout failed: %v", err)
		}
	}()

	buildInfo, err := k.CGInitBuild(request.Name, request.Version, request.Release)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Could not initialize build with koji: %v", err))
	}

	job := worker.OSBuildJob{
		Manifest: ir.manifest,
		Targets: []*target.Target{
			target.NewKojiTarget(&target.KojiTargetOptions{
				BuildID:         uint64(buildInfo.BuildID),
				TaskID:          uint64(request.Koji.TaskId),
				Token:           buildInfo.Token,
				Name:            request.Name,
				Version:         request.Version,
				Release:         request.Release,
				Filename:        ir.filename,
				UploadDirectory: "osbuild-composer-koji-" + uuid.New().String(),
				Server:          request.Koji.Server,
				KojiFilename:    ir.kojiFilename,
			}),
		},
	}

	id, err := h.server.workers.Enqueue(ir.arch, &job)
	if err != nil {
		// This is a programming errror.
		panic(err)
	}

	return ctx.JSON(http.StatusCreated, &api.ComposeResponse{
		Id:          id.String(),
		KojiBuildId: buildInfo.BuildID,
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

func composeStatusFromJobStatus(js *worker.JobStatus, result *worker.OSBuildJobResult) string {
	if js.Canceled {
		return "failure"
	}

	if js.Started.IsZero() {
		return "pending"
	}

	if js.Finished.IsZero() {
		return "pending"
	}

	if result.Success {
		return "success"
	}

	return "failure"
}

func imageStatusFromJobStatus(js *worker.JobStatus, result *worker.OSBuildJobResult) string {
	if js.Canceled {
		return "failure"
	}

	if js.Started.IsZero() {
		return "pending"
	}

	if js.Finished.IsZero() {
		return "building"
	}

	if result.Success {
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

	var result worker.OSBuildJobResult
	status, err := h.server.workers.JobStatus(id, &result)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Job %s not found: %s", idstr, err))
	}

	response := api.ComposeStatus{
		Status: composeStatusFromJobStatus(status, &result),
		ImageStatuses: []api.ImageStatus{
			{
				Status: imageStatusFromJobStatus(status, &result),
			},
		},
	}
	return ctx.JSON(http.StatusOK, response)
}

// GetStatus handles a /status GET request
func (h *apiHandlers) GetStatus(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, &api.Status{
		Status: "OK",
	})
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
