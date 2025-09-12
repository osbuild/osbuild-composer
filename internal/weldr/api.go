package weldr

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gobwas/glob"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/osbuild/osbuild-composer/pkg/jobqueue"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/disk/partition"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/distroidparser"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rhsm/facts"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

// Solver interface defines the methods required for dependency solving by the API
type Solver interface {
	CleanCache() error
	Depsolve(pkgSets []rpmmd.PackageSet, sbomType sbom.StandardType) (*depsolvednf.DepsolveResult, error)
	FetchMetadata(repos []rpmmd.RepoConfig) (rpmmd.PackageList, error)
	SearchMetadata(repos []rpmmd.RepoConfig, packages []string) (rpmmd.PackageList, error)
}

// GetSolverFn is a function type that returns a Solver instance based on the provided parameters
type GetSolverFn func(modulePlatformID, releaseVer, arch, distro string) Solver

type API struct {
	store   *store.Store
	workers *worker.Server

	getSolver    GetSolverFn
	hostArch     string
	repoRegistry *reporegistry.RepoRegistry

	logger *log.Logger
	router *httprouter.Router
	server http.Server

	compatOutputDir string

	hostDistroName string                 // Name of the host distro
	distroFactory  *distrofactory.Factory // Available distros

	//  List of ImageType names, which should not be exposed by the API
	distrosImageTypeDenylist map[string][]string
}

type ComposeState int

const (
	ComposeWaiting ComposeState = iota
	ComposeRunning
	ComposeFinished
	ComposeFailed
)

// ToString converts ImageBuildState into a human readable string
func (cs ComposeState) ToString() string {
	switch cs {
	case ComposeWaiting:
		return "WAITING"
	case ComposeRunning:
		return "RUNNING"
	case ComposeFinished:
		return "FINISHED"
	case ComposeFailed:
		return "FAILED"
	default:
		panic("invalid ComposeState value")
	}
}

// systemRepoNames returns a list of the system repos
// NOTE: The system repos have no concept of id vs. name so the id is returned
func (api *API) systemRepoNames() (names []string) {
	repos, err := api.repoRegistry.ReposByArchName(api.hostDistroName, api.hostArch, false)
	if err == nil {
		for _, repo := range repos {
			names = append(names, repo.Name)
		}
	}
	return names
}

// validDistros returns a sorted list of distributions that also have
// repositories defined for the given architecture
func (api *API) validDistros(arch string) []string {
	distros := []string{}
	for _, distroName := range api.repoRegistry.ListDistros() {
		distro := api.distroFactory.GetDistro(distroName)
		if distro == nil {
			if api.logger != nil {
				api.logger.Printf("Distro %s has repositories defined, but it's not supported. Skipping.", distroName)
			}
			continue
		}

		_, err := api.repoRegistry.DistroHasRepos(distroName, arch)
		if err == nil {
			distros = append(distros, distroName)
		} else {
			if api.logger != nil {
				api.logger.Printf("Distro %s has no repositories defined for %s architecture, skipping.", distroName, arch)
			}
		}
	}

	sort.Strings(distros)
	return distros
}

var ValidBlueprintName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// NewTestAPI is used for the test framework, sets up a single distro
func NewTestAPI(getSolver GetSolverFn, rr *reporegistry.RepoRegistry, logger *log.Logger, storeFixture *store.Fixture,
	workers *worker.Server, compatOutputDir string, distrosImageTypeDenylist map[string][]string) *API {

	api := &API{
		store:                    storeFixture.Store,
		workers:                  workers,
		getSolver:                getSolver,
		hostArch:                 storeFixture.HostArchName,
		repoRegistry:             rr,
		logger:                   logger,
		compatOutputDir:          compatOutputDir,
		hostDistroName:           storeFixture.HostDistroName,
		distroFactory:            storeFixture.Factory,
		distrosImageTypeDenylist: distrosImageTypeDenylist,
	}
	return setupRouter(api)
}

func New(rr *reporegistry.RepoRegistry, stateDir string, solver *depsolvednf.BaseSolver, df *distrofactory.Factory,
	logger *log.Logger, workers *worker.Server, distrosImageTypeDenylist map[string][]string) (*API, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "", 0)
	}

	hostDistroName, err := distro.GetHostDistroName()
	if err != nil {
		return nil, fmt.Errorf("failed to read host distro information")
	}
	hostArch := arch.Current().String()

	hostDistro := df.GetDistro(hostDistroName)
	if hostDistro != nil {
		// get canonical distro name if the host distro is supported
		hostDistroName = hostDistro.Name()

		_, err = hostDistro.GetArch(hostArch)
		if err != nil {
			return nil, fmt.Errorf("Host distro does not support host architecture: %v", err)
		}

		// Check if repositories for the host distro and arch were loaded
		_, err = rr.ReposByArchName(hostDistroName, hostArch, false)
		if err != nil {
			log.Printf("loaded repository definitions don't contain any for the host distro/arch: %v", err)
		}

	} else {
		log.Printf("host distro %q is not supported: only cross-distro builds are available", hostDistroName)
	}

	store := store.New(&stateDir, df, logger)
	compatOutputDir := path.Join(stateDir, "outputs")

	api := &API{
		store:   store,
		workers: workers,
		getSolver: func(modulePlatformID, releaseVer, arch, distro string) Solver {
			return solver.NewWithConfig(modulePlatformID, releaseVer, arch, distro)
		},
		hostArch:                 hostArch,
		repoRegistry:             rr,
		logger:                   logger,
		compatOutputDir:          compatOutputDir,
		hostDistroName:           hostDistroName,
		distroFactory:            df,
		distrosImageTypeDenylist: distrosImageTypeDenylist,
	}
	return setupRouter(api), nil
}

func setupRouter(api *API) *API {
	api.router = httprouter.New()
	api.router.RedirectTrailingSlash = false
	api.router.RedirectFixedPath = false
	api.router.MethodNotAllowed = http.HandlerFunc(methodNotAllowedHandler)
	api.router.NotFound = http.HandlerFunc(notFoundHandler)

	api.router.GET("/api/status", api.statusHandler)
	api.router.GET("/api/v:version/projects/source/list", api.sourceListHandler)
	api.router.GET("/api/v:version/projects/source/info/", api.sourceEmptyInfoHandler)
	api.router.GET("/api/v:version/projects/source/info/:sources", api.sourceInfoHandler)
	api.router.POST("/api/v:version/projects/source/new", api.sourceNewHandler)
	api.router.DELETE("/api/v:version/projects/source/delete/*source", api.sourceDeleteHandler)

	api.router.GET("/api/v:version/projects/depsolve", api.projectsDepsolveHandler)
	api.router.GET("/api/v:version/projects/depsolve/*projects", api.projectsDepsolveHandler)

	api.router.GET("/api/v:version/modules/list", api.modulesListHandler)
	api.router.GET("/api/v:version/modules/list/*modules", api.modulesListHandler)
	api.router.GET("/api/v:version/projects/list", api.projectsListHandler)
	api.router.GET("/api/v:version/projects/list/", api.projectsListHandler)

	// these are the same, except that modules/info also includes dependencies
	api.router.GET("/api/v:version/modules/info", api.modulesInfoHandler)
	api.router.GET("/api/v:version/modules/info/*modules", api.modulesInfoHandler)
	api.router.GET("/api/v:version/projects/info", api.modulesInfoHandler)
	api.router.GET("/api/v:version/projects/info/*modules", api.modulesInfoHandler)

	api.router.GET("/api/v:version/blueprints/list", api.blueprintsListHandler)
	api.router.GET("/api/v:version/blueprints/info/*blueprints", api.blueprintsInfoHandler)
	api.router.GET("/api/v:version/blueprints/depsolve/*blueprints", api.blueprintsDepsolveHandler)
	api.router.GET("/api/v:version/blueprints/freeze/*blueprints", api.blueprintsFreezeHandler)
	api.router.GET("/api/v:version/blueprints/diff/:blueprint/:from/:to", api.blueprintsDiffHandler)
	api.router.GET("/api/v:version/blueprints/change/:blueprint/:commit", api.blueprintsChangeHandler)
	api.router.GET("/api/v:version/blueprints/changes/*blueprints", api.blueprintsChangesHandler)
	api.router.POST("/api/v:version/blueprints/new", api.blueprintsNewHandler)
	api.router.POST("/api/v:version/blueprints/workspace", api.blueprintsWorkspaceHandler)
	api.router.POST("/api/v:version/blueprints/undo/:blueprint/:commit", api.blueprintUndoHandler)
	api.router.POST("/api/v:version/blueprints/tag/:blueprint", api.blueprintsTagHandler)
	api.router.DELETE("/api/v:version/blueprints/delete/:blueprint", api.blueprintDeleteHandler)
	api.router.DELETE("/api/v:version/blueprints/workspace/:blueprint", api.blueprintDeleteWorkspaceHandler)

	api.router.POST("/api/v:version/compose", api.composeHandler)
	api.router.DELETE("/api/v:version/compose/delete/:uuids", api.composeDeleteHandler)
	api.router.GET("/api/v:version/compose/types", api.composeTypesHandler)
	api.router.GET("/api/v:version/compose/queue", api.composeQueueHandler)
	api.router.GET("/api/v:version/compose/status/:uuids", api.composeStatusHandler)
	api.router.GET("/api/v:version/compose/info/:uuid", api.composeInfoHandler)
	api.router.GET("/api/v:version/compose/finished", api.composeFinishedHandler)
	api.router.GET("/api/v:version/compose/failed", api.composeFailedHandler)
	api.router.GET("/api/v:version/compose/image/:uuid", api.composeImageHandler)
	api.router.GET("/api/v:version/compose/metadata/:uuid", api.composeMetadataHandler)
	api.router.GET("/api/v:version/compose/results/:uuid", api.composeResultsHandler)
	api.router.GET("/api/v:version/compose/logs/:uuid", api.composeLogsHandler)
	api.router.GET("/api/v:version/compose/log/:uuid", api.composeLogHandler)
	api.router.POST("/api/v:version/compose/uploads/schedule/:uuid", api.uploadsScheduleHandler)
	api.router.DELETE("/api/v:version/compose/cancel/:uuid", api.composeCancelHandler)

	api.router.DELETE("/api/v:version/upload/delete/:uuid", api.uploadsDeleteHandler)
	api.router.GET("/api/v:version/upload/info/:uuid", api.uploadsInfoHandler)
	api.router.GET("/api/v:version/upload/log/:uuid", api.uploadsLogHandler)
	api.router.POST("/api/v:version/upload/reset/:uuid", api.uploadsResetHandler)
	api.router.DELETE("/api/v:version/upload/cancel/:uuid", api.uploadsCancelHandler)

	api.router.GET("/api/v:version/upload/providers", api.providersHandler)
	api.router.POST("/api/v:version/upload/providers/save", api.providersSaveHandler)
	api.router.DELETE("/api/v:version/upload/providers/delete/:provider/:profile", api.providersDeleteHandler)

	api.router.GET("/api/v:version/distros/list", api.distrosListHandler)
	return api
}

func (api *API) Serve(listener net.Listener) error {
	api.server = http.Server{
		Handler:           api,
		ReadHeaderTimeout: 5 * time.Second,
	}

	err := api.server.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (api *API) Shutdown(ctx context.Context) error {
	return api.server.Shutdown(ctx)
}

func (api *API) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if api.logger != nil {
		log.Println(request.Method, request.URL.Path)
	}

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	api.router.ServeHTTP(writer, request)
}

// PreloadMetadata loads the metadata for all supported distros
// This starts a background depsolve for all known distros in order to preload the
// metadata.
func (api *API) PreloadMetadata() {
	log.Printf("Starting metadata preload goroutines")
	for _, distro := range api.validDistros(api.hostArch) {
		go func(distro string) {
			startTime := time.Now()
			d := api.getDistro(distro, api.hostArch)
			if d == nil {
				log.Printf("GetDistro - unknown distribution: %s", distro)
				return
			}

			repos, err := api.allRepositories(distro, api.hostArch)
			if err != nil {
				log.Printf("Error getting repositories for distro %s: %s", distro, err)
				return
			}

			solver := api.getSolver(d.ModulePlatformID(), d.Releasever(), api.hostArch, d.Name())
			_, err = solver.Depsolve([]rpmmd.PackageSet{{Include: []string{"filesystem"}, Repositories: repos}}, sbom.StandardTypeNone)
			if err != nil {
				log.Printf("Problem preloading distro metadata for %s: %s", distro, err)
			}
			log.Printf("Finished preload of %s in %v", distro, time.Since(startTime))
		}(distro)
	}
}

type composeStatus struct {
	State    ComposeState
	Queued   time.Time
	Started  time.Time
	Finished time.Time
	Result   *osbuild.Result
}

func composeStateFromJobStatus(js *worker.JobStatus, result *worker.OSBuildJobResult) ComposeState {
	if js.Canceled {
		return ComposeFailed
	}

	if js.Started.IsZero() {
		return ComposeWaiting
	}

	if js.Finished.IsZero() {
		return ComposeRunning
	}

	if result.Success {
		return ComposeFinished
	}

	return ComposeFailed
}

// Returns the state of the image in `compose` and the times the job was
// queued, started, and finished. Assumes that there's only one image in the
// compose.
func (api *API) getComposeStatus(compose store.Compose) (*composeStatus, error) {
	jobId := compose.ImageBuild.JobID

	// backwards compatibility: composes that were around before splitting
	// the job queue from the store still contain their valid status and
	// times. Return those here as a fallback.
	if jobId == uuid.Nil {
		var state ComposeState
		switch compose.ImageBuild.QueueStatus {
		case common.IBWaiting:
			state = ComposeWaiting
		case common.IBRunning:
			state = ComposeRunning
		case common.IBFinished:
			state = ComposeFinished
		case common.IBFailed:
			state = ComposeFailed
		}
		return &composeStatus{
			State:    state,
			Queued:   compose.ImageBuild.JobCreated,
			Started:  compose.ImageBuild.JobStarted,
			Finished: compose.ImageBuild.JobFinished,
			Result:   &osbuild.Result{},
		}, nil
	}

	// All jobs are "osbuild" jobs.
	var result worker.OSBuildJobResult
	jobInfo, err := api.workers.OSBuildJobInfo(jobId, &result)
	if err != nil {
		return nil, err
	}

	return &composeStatus{
		State:    composeStateFromJobStatus(jobInfo.JobStatus, &result),
		Queued:   jobInfo.JobStatus.Queued,
		Started:  jobInfo.JobStatus.Started,
		Finished: jobInfo.JobStatus.Finished,
		Result:   result.OSBuildOutput,
	}, nil
}

// Opens the image file for `compose`. This asks the worker server for the
// artifact first, and then falls back to looking in
// `{outputs}/{composeId}/{imageBuildId}` for backwards compatibility.
func (api *API) openImageFile(composeId uuid.UUID, compose store.Compose) (io.Reader, int64, error) {
	name := compose.ImageBuild.ImageType.Filename()

	reader, size, err := api.workers.JobArtifact(compose.ImageBuild.JobID, name)
	if err != nil {
		if api.compatOutputDir == "" || err != jobqueue.ErrNotExist {
			return nil, 0, err
		}

		p := path.Join(api.compatOutputDir, composeId.String(), "0", name)
		f, err := os.Open(p)
		if err != nil {
			return nil, 0, err
		}

		info, err := f.Stat()
		if err != nil {
			return nil, 0, err
		}

		reader = f
		size = info.Size()
	}

	return reader, size, nil
}

// isImageTypeAllowed checks the given ImageType and Distro names against
// the distro-specific ImageType Denylist provided to the API from configuration.
// If the given ImageType is not allowed the method returns an `false`.
// Otherwise `true` is returned.
func (api *API) isImageTypeAllowed(distroName, imageType string) (bool, error) {
	for deniedDistro, deniedImgTypes := range api.distrosImageTypeDenylist {
		deniedDistroPattern, err := glob.Compile(deniedDistro)
		if err != nil {
			// the bool return value here does not have any real meaning
			return true, err
		}
		if deniedDistroPattern.Match(distroName) {
			for _, deniedImgType := range deniedImgTypes {
				deniedImageTypePattern, err := glob.Compile(deniedImgType)
				if err != nil {
					// the bool return value here does not have any real meaning
					return true, err
				}

				if deniedImageTypePattern.Match(imageType) {
					return false, nil
				}
			}
		}
	}

	return true, nil
}

// getImageType returns the ImageType for the selected distro
// This is necessary because different distros support different image types, and the image
// type may have a different package set than other distros.
func (api *API) getImageType(distroName, imageType, archName string) (distro.ImageType, error) {
	imgAllowed, err := api.isImageTypeAllowed(distroName, imageType)
	if err != nil {
		return nil, fmt.Errorf("error while checking if image type is allowed: %v", err)
	}
	if !imgAllowed {
		return nil, fmt.Errorf("image type %q for distro %q is denied by configuration", imageType, distroName)
	}

	distro := api.getDistro(distroName, archName)
	if distro == nil {
		return nil, fmt.Errorf("GetDistro - unknown distribution: %s", distroName)
	}

	arch, err := distro.GetArch(archName)
	if err != nil {
		return nil, err
	}
	return arch.GetImageType(imageType)
}

func (api *API) parseDistro(query url.Values, arch string) (string, error) {
	if distro := query.Get("distro"); distro != "" {
		if common.IsStringInSortedSlice(api.validDistros(arch), distro) {
			return distro, nil
		}
		return "", errors.New("Invalid distro: " + distro)
	}
	return api.hostDistroName, nil
}

// getDistro returns the named distro or nil
// It excludes unsupported distros by first checking the api.distros list
func (api *API) getDistro(name, arch string) distro.Distro {
	if !common.IsStringInSortedSlice(api.validDistros(arch), name) {
		return nil
	}
	return api.distroFactory.GetDistro(name)
}

func verifyRequestVersion(writer http.ResponseWriter, params httprouter.Params, minVersion uint) bool {
	versionString := params.ByName("version")

	version, err := strconv.ParseUint(versionString, 10, 0)

	var MaxApiVersion uint = 1

	if err != nil || uint(version) < minVersion || uint(version) > MaxApiVersion {
		notFoundHandler(writer, nil)
		return false
	}

	return true
}

func isRequestVersionAtLeast(params httprouter.Params, minVersion uint) bool {
	versionString := params.ByName("version")

	version, err := strconv.ParseUint(versionString, 10, 0)

	common.PanicOnError(err)

	return uint(version) >= minVersion
}

func methodNotAllowedHandler(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusMethodNotAllowed)
}

func notFoundHandler(writer http.ResponseWriter, request *http.Request) {
	errors := responseError{
		Code: http.StatusNotFound,
		ID:   "HTTPError",
		Msg:  "Not Found",
	}
	statusResponseError(writer, http.StatusNotFound, errors)
}

func notImplementedHandler(writer http.ResponseWriter, httpRequest *http.Request, _ httprouter.Params) {
	writer.WriteHeader(http.StatusNotImplemented)
}

func statusResponseOK(writer http.ResponseWriter) {
	type reply struct {
		Status bool `json:"status"`
	}

	writer.WriteHeader(http.StatusOK)
	err := json.NewEncoder(writer).Encode(reply{true})
	common.PanicOnError(err)
}

type responseError struct {
	Code int    `json:"code,omitempty"`
	ID   string `json:"id"`
	Msg  string `json:"msg"`
}

// verifyStringsWithRegex checks a slice of strings against a regex of allowed characters
// it writes the InvalidChars error to the writer and returns false if any of them fail the check
// It will also return an error if the string is empty
func verifyStringsWithRegex(writer http.ResponseWriter, strings []string, re *regexp.Regexp) bool {
	for _, s := range strings {
		if len(s) > 0 && re.MatchString(s) {
			continue
		}
		errors := responseError{
			ID:  "InvalidChars",
			Msg: "Invalid characters in API path",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return false
	}
	return true
}

func statusResponseError(writer http.ResponseWriter, code int, errors ...responseError) {
	type reply struct {
		Status bool            `json:"status"`
		Errors []responseError `json:"errors,omitempty"`
	}

	writer.WriteHeader(code)
	err := json.NewEncoder(writer).Encode(reply{false, errors})
	common.PanicOnError(err)
}

func (api *API) statusHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	type reply struct {
		API           string   `json:"api"`
		DBSupported   bool     `json:"db_supported"`
		DBVersion     string   `json:"db_version"`
		SchemaVersion string   `json:"schema_version"`
		Backend       string   `json:"backend"`
		Build         string   `json:"build"`
		Messages      []string `json:"msgs"`
	}

	err := json.NewEncoder(writer).Encode(reply{
		API:           "1",
		DBSupported:   true,
		DBVersion:     "0",
		SchemaVersion: "0",
		Backend:       "osbuild-composer",
		Build:         common.BuildVersion(),
		Messages:      make([]string, 0),
	})
	common.PanicOnError(err)
}

func (api *API) sourceListHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type reply struct {
		Sources []string `json:"sources"`
	}

	// The v0 API used the repo Name, a descriptive string, as the key
	// In the v1 API this was changed to separate the Name and the Id (a short identifier)
	var names []string
	if isRequestVersionAtLeast(params, 1) {
		names = api.store.ListSourcesById()
	} else {
		names = api.store.ListSourcesByName()
	}
	names = append(names, api.systemRepoNames()...)

	err := json.NewEncoder(writer).Encode(reply{
		Sources: names,
	})
	common.PanicOnError(err)
}

func (api *API) sourceEmptyInfoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	errors := responseError{
		Code: http.StatusNotFound,
		ID:   "HTTPError",
		Msg:  "Not Found",
	}
	statusResponseError(writer, http.StatusNotFound, errors)
}

// getSourceConfigs retrieves the list of sources from the system repos an store
// Returning a list of store.SourceConfig entries indexed by the id of the source
func (api *API) getSourceConfigs(params httprouter.Params) (map[string]store.SourceConfig, []responseError) {
	names := params.ByName("sources")

	sources := map[string]store.SourceConfig{}
	errors := []responseError{}

	repos, err := api.repoRegistry.ReposByArchName(api.hostDistroName, api.hostArch, false)
	if err != nil {
		error := responseError{
			ID:  "InternalError",
			Msg: fmt.Sprintf("error while getting system repos: %v", err),
		}
		errors = append(errors, error)
	}

	// if names is "*" we want all sources
	if names == "*" {
		sources = api.store.GetAllSourcesByID()
		for _, repo := range repos {
			sources[repo.Name] = store.NewSourceConfig(repo, true)
		}
	} else {
		for _, name := range strings.Split(names, ",") {
			// check if the source is one of the base repos
			found := false
			for _, repo := range repos {
				if name == repo.Name {
					sources[repo.Name] = store.NewSourceConfig(repo, true)
					found = true
					break
				}
			}
			if found {
				continue
			}
			// check if the source is in the store
			if source := api.store.GetSource(name); source != nil {
				sources[name] = *source
			} else {
				error := responseError{
					ID:  "UnknownSource",
					Msg: fmt.Sprintf("%s is not a valid source", name),
				}
				errors = append(errors, error)
			}
		}
	}

	return sources, errors
}

// sourceInfoHandler routes the call to the correct API version handler
func (api *API) sourceInfoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	versionString := params.ByName("version")
	version, err := strconv.ParseUint(versionString, 10, 0)
	if err != nil {
		notFoundHandler(writer, nil)
		return
	}
	switch version {
	case 0:
		api.sourceInfoHandlerV0(writer, request, params)
	case 1:
		api.sourceInfoHandlerV1(writer, request, params)
	default:
		notFoundHandler(writer, nil)
	}
}

// sourceInfoHandlerV0 handles the API v0 response
func (api *API) sourceInfoHandlerV0(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	sources, errors := api.getSourceConfigs(params)

	// V0 responses use the source name as the key
	v0Sources := make(map[string]SourceConfigV0, len(sources))
	for _, s := range sources {
		v0Sources[s.Name] = NewSourceConfigV0(s)
	}

	q, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		errors := responseError{
			ID:  "InvalidChars",
			Msg: fmt.Sprintf("invalid query string: %v", err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	format := q.Get("format")
	if format == "json" || format == "" {
		err := json.NewEncoder(writer).Encode(SourceInfoResponseV0{
			Sources: v0Sources,
			Errors:  errors,
		})
		common.PanicOnError(err)
	} else if format == "toml" {
		encoder := toml.NewEncoder(writer)
		encoder.Indent = ""
		err := encoder.Encode(sources)
		common.PanicOnError(err)
	} else {
		errors := responseError{
			ID:  "InvalidChars",
			Msg: fmt.Sprintf("invalid format parameter: %s", format),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
	}
}

// sourceInfoHandlerV1 handles the API v0 response
func (api *API) sourceInfoHandlerV1(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	sources, errors := api.getSourceConfigs(params)

	// V1 responses use the source id as the key
	v1Sources := make(map[string]SourceConfigV1, len(sources))
	for id, s := range sources {
		v1Sources[id] = NewSourceConfigV1(id, s)
	}

	q, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		errors := responseError{
			ID:  "InvalidChars",
			Msg: fmt.Sprintf("invalid query string: %v", err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	format := q.Get("format")
	if format == "json" || format == "" {
		err := json.NewEncoder(writer).Encode(SourceInfoResponseV1{
			Sources: v1Sources,
			Errors:  errors,
		})
		common.PanicOnError(err)
	} else if format == "toml" {
		encoder := toml.NewEncoder(writer)
		encoder.Indent = ""
		err := encoder.Encode(sources)
		common.PanicOnError(err)
	} else {
		errors := responseError{
			ID:  "InvalidChars",
			Msg: fmt.Sprintf("invalid format parameter: %s", format),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
	}
}

// DecodeSourceConfigV0 parses a request.Body into a SourceConfigV0
func DecodeSourceConfigV0(body io.Reader, contentType string) (source SourceConfigV0, err error) {
	if contentType == "application/json" {
		err = json.NewDecoder(body).Decode(&source)
	} else if contentType == "text/x-toml" {
		// Read all of body in case it needs to be parsed twice
		var data []byte
		data, err = io.ReadAll(body)
		if err != nil {
			return source, err
		}

		// First try to parse [ID]\n...SOURCE... form
		var srcMap map[string]SourceConfigV0
		err = toml.Unmarshal(data, &srcMap)
		if err != nil {
			// Failed, parse it again without [ID] mapping
			err = toml.Unmarshal(data, &source)
		} else {
			// It is possible more than 1 source could be posted. We only use the first, after sorting
			keys := make([]string, 0, len(srcMap))
			for k := range srcMap {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			source = srcMap[keys[0]]
		}
	} else {
		err = errors.New("blueprint must be in json or toml format")
	}
	return source, err
}

// DecodeSourceConfigV1 parses a request.Body into a SourceConfigV1
func DecodeSourceConfigV1(body io.Reader, contentType string) (source SourceConfigV1, err error) {
	if contentType == "application/json" {
		err = json.NewDecoder(body).Decode(&source)
	} else if contentType == "text/x-toml" {
		// Read all of body in case it needs to be parsed twice
		var data []byte
		data, err = io.ReadAll(body)
		if err != nil {
			return source, err
		}

		// First try to parse [ID]\n...SOURCE... form
		var srcMap map[string]SourceConfigV1
		err = toml.Unmarshal(data, &srcMap)
		if err != nil {
			// Failed, parse it without [ID]
			err = toml.Unmarshal(data, &source)
		} else {
			// It is possible more than 1 source could be posted. We only use the first, after sorting
			keys := make([]string, 0, len(srcMap))
			for k := range srcMap {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			source = srcMap[keys[0]]

			// If no id was set, use the ID from the map
			if len(source.ID) == 0 {
				source.ID = keys[0]
			}
		}
	} else {
		err = errors.New("blueprint must be in json or toml format")
	}

	if err == nil && len(source.GetKey()) == 0 {
		err = errors.New("'id' field is missing from request")
	}

	return source, err
}

func (api *API) sourceNewHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) { // TODO: version 1 API
		return
	}

	contentType := request.Header["Content-Type"]
	if len(contentType) == 0 {
		errors := responseError{
			ID:  "HTTPError",
			Msg: "malformed Content-Type header",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	if request.ContentLength == 0 {
		errors := responseError{
			ID:  "ProjectsError",
			Msg: "Missing source",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	var source SourceConfig
	var err error
	if isRequestVersionAtLeast(params, 1) {
		source, err = DecodeSourceConfigV1(request.Body, contentType[0])
	} else {
		source, err = DecodeSourceConfigV0(request.Body, contentType[0])
	}

	// Basic check of the source, should at least have a name and type
	if err == nil {
		if len(source.GetName()) == 0 {
			err = errors.New("'name' field is missing from request")
		} else if len(source.GetType()) == 0 {
			err = errors.New("'type' field is missing from request")
		} else if len(source.SourceConfig().URL) == 0 {
			err = errors.New("'url' field is missing from request")
		}
	}
	if err != nil {
		errors := responseError{
			ID:  "ProjectsError",
			Msg: "Problem parsing POST body: " + err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	// If there is a list of distros, check to make sure they are valid
	invalid := []string{}
	for _, d := range source.SourceConfig().Distros {
		if !common.IsStringInSortedSlice(api.validDistros(api.hostArch), d) {
			invalid = append(invalid, d)
		}
	}
	if len(invalid) > 0 {
		errors := responseError{
			ID:  "ProjectsError",
			Msg: "Invalid distributions: " + strings.Join(invalid, ","),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	// Is there an existing System Repo using this id?
	for _, n := range api.systemRepoNames() {
		if n == source.GetKey() {
			// Users cannot replace system repos
			errors := responseError{
				ID:  "SystemSource",
				Msg: fmt.Sprintf("%s is a system source, it cannot be changed.", source.GetKey()),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}
	}

	api.store.PushSource(source.GetKey(), source.SourceConfig())

	statusResponseOK(writer)
}

func (api *API) sourceDeleteHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	name := strings.Split(params.ByName("source"), ",")

	if name[0] == "/" {
		errors := responseError{
			Code: http.StatusNotFound,
			ID:   "HTTPError",
			Msg:  "Not Found",
		}
		statusResponseError(writer, http.StatusNotFound, errors)
		return
	}

	// Check for system repos and return an error
	for _, id := range api.systemRepoNames() {
		if id == name[0][1:] {
			errors := responseError{
				ID:  "SystemSource",
				Msg: id + " is a system source, it cannot be deleted.",
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}
	}

	// Return an error for unknown sources
	s := api.store.GetSource(name[0][1:])
	if s == nil {
		errors := responseError{
			ID:  "UnknownSource",
			Msg: name[0][1:] + " is not a valid source.",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	// Only delete the first name, which will have a / at the start because of the /*source route
	if isRequestVersionAtLeast(params, 1) {
		api.store.DeleteSourceByID(name[0][1:])
	} else {
		api.store.DeleteSourceByName(name[0][1:])
	}

	statusResponseOK(writer)
}

func (api *API) modulesListHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type module struct {
		Name      string `json:"name"`
		GroupType string `json:"group_type"`
	}

	type reply struct {
		Total   uint     `json:"total"`
		Offset  uint     `json:"offset"`
		Limit   uint     `json:"limit"`
		Modules []module `json:"modules"`
	}

	offset, limit, err := parseOffsetAndLimit(request.URL.Query())
	if err != nil {
		errors := responseError{
			ID:  "BadLimitOrOffset",
			Msg: fmt.Sprintf("BadRequest: %s", err.Error()),
		}

		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	modulesParam := params.ByName("modules")

	archName := api.hostArch
	if queryArch := request.URL.Query().Get("arch"); queryArch != "" {
		archName = queryArch
	}

	// Optional distro parameter
	// If it is empty it will return api.hostDistroName
	distroName, err := api.parseDistro(request.URL.Query(), archName)
	if err != nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	var names []string
	if modulesParam != "" && modulesParam != "/" {
		// we have modules for search

		// remove leading /
		modulesParam = modulesParam[1:]

		names = strings.Split(modulesParam, ",")
	}
	packages, err := api.fetchPackageList(distroName, archName, names)

	if err != nil {
		errors := responseError{
			ID:  "ModulesError",
			Msg: fmt.Sprintf("msg: %s", err.Error()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	if len(packages) == 0 {
		errors := responseError{
			ID:  "UnknownModule",
			Msg: "No packages have been found.",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	packageInfos := packages.ToPackageInfos()

	total := uint(len(packageInfos))
	start := min(offset, total)
	n := min(limit, total-start)

	modules := make([]module, n)
	for i := uint(0); i < n; i++ {
		modules[i] = module{packageInfos[start+i].Name, "rpm"}
	}

	err = json.NewEncoder(writer).Encode(reply{
		Total:   total,
		Offset:  offset,
		Limit:   limit,
		Modules: modules,
	})
	common.PanicOnError(err)
}

func (api *API) projectsListHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type reply struct {
		Total    uint                `json:"total"`
		Offset   uint                `json:"offset"`
		Limit    uint                `json:"limit"`
		Projects []rpmmd.PackageInfo `json:"projects"`
	}

	offset, limit, err := parseOffsetAndLimit(request.URL.Query())
	if err != nil {
		errors := responseError{
			ID:  "BadLimitOrOffset",
			Msg: fmt.Sprintf("BadRequest: %s", err.Error()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	archName := api.hostArch
	if queryArch := request.URL.Query().Get("arch"); queryArch != "" {
		archName = queryArch
	}

	// Optional distro parameter
	// If it is empty it will return api.hostDistroName
	distroName, err := api.parseDistro(request.URL.Query(), archName)
	if err != nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
	availablePackages, err := api.fetchPackageList(distroName, archName, []string{})

	if err != nil {
		errors := responseError{
			ID:  "ProjectsError",
			Msg: fmt.Sprintf("msg: %s", err.Error()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	packageInfos := availablePackages.ToPackageInfos()

	total := uint(len(packageInfos))
	start := min(offset, total)
	n := min(limit, total-start)

	packages := make([]rpmmd.PackageInfo, n)
	for i := uint(0); i < n; i++ {
		packages[i] = packageInfos[start+i]
	}

	err = json.NewEncoder(writer).Encode(reply{
		Total:    total,
		Offset:   offset,
		Limit:    limit,
		Projects: packages,
	})
	common.PanicOnError(err)
}

func (api *API) modulesInfoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type projectsReply struct {
		Projects []rpmmd.PackageInfo `json:"projects"`
	}
	type modulesReply struct {
		Modules []rpmmd.PackageInfo `json:"modules"`
	}

	// handle both projects/info and modules/info, the latter includes dependencies
	modulesRequested := strings.HasPrefix(request.URL.Path, "/api/v0/modules") ||
		strings.HasPrefix(request.URL.Path, "/api/v1/modules")

	var errorId, unknownErrorId string
	if modulesRequested {
		errorId = "ModulesError"
		unknownErrorId = "UnknownModule"
	} else {
		errorId = "ProjectsError"
		unknownErrorId = "UnknownProject"
	}

	modules := params.ByName("modules")

	if modules == "" || modules == "/" {
		errors := responseError{
			ID:  unknownErrorId,
			Msg: "No packages specified.",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	// remove leading /
	modules = modules[1:]

	names := strings.Split(modules, ",")

	archName := api.hostArch
	if queryArch := request.URL.Query().Get("arch"); queryArch != "" {
		archName = queryArch
	}

	// Optional distro parameter
	// If it is empty it will return api.hostDistroName
	distroName, err := api.parseDistro(request.URL.Query(), archName)
	if err != nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
	foundPackages, err := api.fetchPackageList(distroName, archName, names)

	if err != nil {
		errors := responseError{
			ID:  errorId,
			Msg: fmt.Sprintf("msg: %s", err.Error()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	if len(foundPackages) == 0 {
		errors := responseError{
			ID:  unknownErrorId,
			Msg: "No packages have been found.",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	packageInfos := foundPackages.ToPackageInfos()

	if modulesRequested {
		repos, err := api.allRepositories(distroName, archName)
		if err != nil {
			errors := responseError{
				ID:  "InternalError",
				Msg: fmt.Sprintf("error while getting system repos: %v", err),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}
		d := api.getDistro(distroName, archName)
		if d == nil {
			errors := responseError{
				ID:  "DistroError",
				Msg: fmt.Sprintf("Unknown distribution: %s", distroName),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}

		solver := api.getSolver(d.ModulePlatformID(), d.Releasever(), archName, d.Name())
		for i := range packageInfos {
			pkgName := packageInfos[i].Name
			res, err := solver.Depsolve([]rpmmd.PackageSet{{Include: []string{pkgName}, Repositories: repos}}, sbom.StandardTypeNone)
			if err != nil {
				errors := responseError{
					ID:  errorId,
					Msg: fmt.Sprintf("Cannot depsolve package %s: %s", packageInfos[i].Name, err.Error()),
				}
				statusResponseError(writer, http.StatusBadRequest, errors)
				return
			}
			packageInfos[i].Dependencies = res.Packages
		}
		if err := solver.CleanCache(); err != nil {
			// log and ignore
			log.Printf("Error during rpm repo cache cleanup: %s", err.Error())
		}
	}

	if modulesRequested {
		err = json.NewEncoder(writer).Encode(modulesReply{packageInfos})
	} else {
		err = json.NewEncoder(writer).Encode(projectsReply{packageInfos})
	}
	common.PanicOnError(err)
}

func (api *API) projectsDepsolveHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type reply struct {
		Projects []rpmmd.PackageSpec `json:"projects"`
	}

	projects := params.ByName("projects")

	if projects == "" || projects == "/" {
		errors := responseError{
			ID:  "UnknownProject",
			Msg: "No packages specified.",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	// remove leading /
	projects = projects[1:]
	names := strings.Split(projects, ",")

	archName := api.hostArch
	if queryArch := request.URL.Query().Get("arch"); queryArch != "" {
		archName = queryArch
	}

	// Optional distro parameter
	// If it is empty it will return api.hostDistroName
	distroName, err := api.parseDistro(request.URL.Query(), archName)
	if err != nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	d := api.getDistro(distroName, archName)
	if d == nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: fmt.Sprintf("Unknown distribution: %s", distroName),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	repos, err := api.allRepositories(distroName, archName)
	if err != nil {
		errors := responseError{
			ID:  "ProjectsError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	solver := api.getSolver(d.ModulePlatformID(), d.Releasever(), archName, d.Name())
	res, err := solver.Depsolve([]rpmmd.PackageSet{{Include: names, Repositories: repos}}, sbom.StandardTypeNone)
	if err != nil {
		errors := responseError{
			ID:  "ProjectsError",
			Msg: fmt.Sprintf("BadRequest: %s", err.Error()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
	if err := solver.CleanCache(); err != nil {
		// log and ignore
		log.Printf("Error during rpm repo cache cleanup: %s", err.Error())
	}
	err = json.NewEncoder(writer).Encode(reply{Projects: res.Packages})
	common.PanicOnError(err)
}

func (api *API) blueprintsListHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type reply struct {
		Total      uint     `json:"total"`
		Offset     uint     `json:"offset"`
		Limit      uint     `json:"limit"`
		Blueprints []string `json:"blueprints"`
	}

	offset, limit, err := parseOffsetAndLimit(request.URL.Query())
	if err != nil {
		errors := responseError{
			ID:  "BadLimitOrOffset",
			Msg: fmt.Sprintf("BadRequest: %s", err.Error()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	names := api.store.ListBlueprints()
	total := uint(len(names))
	offset = min(offset, total)
	limit = min(limit, total-offset)

	err = json.NewEncoder(writer).Encode(reply{
		Total:      total,
		Offset:     offset,
		Limit:      limit,
		Blueprints: names[offset : offset+limit],
	})
	common.PanicOnError(err)
}

func (api *API) blueprintsInfoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type change struct {
		Changed bool   `json:"changed"`
		Name    string `json:"name"`
	}
	type reply struct {
		Blueprints []blueprint.Blueprint `json:"blueprints"`
		Changes    []change              `json:"changes"`
		Errors     []responseError       `json:"errors"`
	}

	names := strings.Split(params.ByName("blueprints"), ",")
	if names[0] == "/" {
		errors := responseError{
			Code: http.StatusNotFound,
			ID:   "HTTPError",
			Msg:  "Not Found",
		}
		statusResponseError(writer, http.StatusNotFound, errors)
		return
	}

	// Remove the leading / from the first entry (check above ensures it is not just a /
	names[0] = names[0][1:]

	if !verifyStringsWithRegex(writer, names, ValidBlueprintName) {
		return
	}

	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		errors := responseError{
			ID:  "InvalidChars",
			Msg: fmt.Sprintf("invalid query string: %v", err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	blueprints := []blueprint.Blueprint{}
	changes := []change{}
	blueprintErrors := []responseError{}

	for _, name := range names {
		blueprint, changed := api.store.GetBlueprint(name)
		if blueprint == nil {
			blueprintErrors = append(blueprintErrors, responseError{
				ID:  "UnknownBlueprint",
				Msg: fmt.Sprintf("%s: ", name),
			})
			continue
		}
		blueprints = append(blueprints, *blueprint)
		changes = append(changes, change{changed, blueprint.Name})
	}

	format := query.Get("format")
	if format == "json" || format == "" {
		err := json.NewEncoder(writer).Encode(reply{
			Blueprints: blueprints,
			Changes:    changes,
			Errors:     blueprintErrors,
		})
		common.PanicOnError(err)
	} else if format == "toml" {
		// lorax concatenates multiple blueprints with `\n\n` here,
		// which is never useful. Deviate by only returning the first
		// blueprint.
		if len(blueprints) > 1 {
			errors := responseError{
				ID:  "HTTPError",
				Msg: "toml format only supported when requesting one blueprint",
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}
		if len(blueprintErrors) > 0 {
			statusResponseError(writer, http.StatusBadRequest, blueprintErrors...)
			return
		}
		encoder := toml.NewEncoder(writer)
		encoder.Indent = ""
		err := encoder.Encode(blueprints[0])
		common.PanicOnError(err)
	} else {
		errors := responseError{
			ID:  "InvalidChars",
			Msg: fmt.Sprintf("invalid format parameter: %s", format),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
}

func (api *API) blueprintsDepsolveHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type entry struct {
		Blueprint    blueprint.Blueprint `json:"blueprint"`
		Dependencies []rpmmd.PackageSpec `json:"dependencies"`
	}
	type reply struct {
		Blueprints []entry         `json:"blueprints"`
		Errors     []responseError `json:"errors"`
	}

	names := strings.Split(params.ByName("blueprints"), ",")
	if names[0] == "/" {
		errors := responseError{
			Code: http.StatusNotFound,
			ID:   "HTTPError",
			Msg:  "Not Found",
		}
		statusResponseError(writer, http.StatusNotFound, errors)
		return
	}

	// Remove the leading / from the first entry (check above ensures it is not just a /
	names[0] = names[0][1:]

	if !verifyStringsWithRegex(writer, names, ValidBlueprintName) {
		return
	}

	blueprints := []entry{}
	blueprintsErrors := []responseError{}
	for _, name := range names {
		blueprint, _ := api.store.GetBlueprint(name)
		if blueprint == nil {
			blueprintsErrors = append(blueprintsErrors, responseError{
				ID:  "UnknownBlueprint",
				Msg: fmt.Sprintf("%s: blueprint not found", name),
			})
			continue
		}

		dependencies, err := api.depsolveBlueprint(*blueprint)

		if err != nil {
			blueprintsErrors = append(blueprintsErrors, responseError{
				ID:  "BlueprintsError",
				Msg: fmt.Sprintf("%s: %s", name, err.Error()),
			})
			dependencies = []rpmmd.PackageSpec{}
		}

		blueprints = append(blueprints, entry{*blueprint, dependencies})
	}

	err := json.NewEncoder(writer).Encode(reply{
		Blueprints: blueprints,
		Errors:     blueprintsErrors,
	})
	common.PanicOnError(err)
}

// expandBlueprintGlobs will expand package name globs and versions using the depsolve results
// The result is a sorted list of Package structs with the full package name and version
// It will return an error if it cannot find a non-glob package name in the dependency list
func expandBlueprintGlobs(dependencies []rpmmd.PackageSpec, packages []blueprint.Package) ([]blueprint.Package, error) {
	newPackages := make(map[string]blueprint.Package)

	for _, pkg := range packages {
		if !strings.ContainsAny(pkg.Name, "*?") {
			// No glob characters, find an exact match in dependencies
			i := sort.Search(len(dependencies), func(i int) bool {
				return dependencies[i].Name >= pkg.Name
			})
			if i >= len(dependencies) || dependencies[i].Name != pkg.Name {
				// Packages should not be missing from the depsolve results
				return nil, fmt.Errorf("%s missing from depsolve results", pkg.Name)
			}
			newPackages[dependencies[i].GetNEVRA()] = blueprint.Package{
				Name:    dependencies[i].Name,
				Version: dependencies[i].GetEVRA(),
			}
		} else {
			// Add all the packages matching the glob
			g, err := glob.Compile(pkg.Name)
			if err != nil {
				return nil, err
			}

			for _, d := range dependencies {
				if g.Match(d.Name) {
					newPackages[d.GetNEVRA()] = blueprint.Package{
						Name:    d.Name,
						Version: d.GetEVRA(),
					}
				}
			}
		}
	}

	// Return a sorted slice of the Packages
	np := make([]blueprint.Package, 0, len(newPackages))
	for _, pkg := range newPackages {
		np = append(np, pkg)
	}
	sort.Slice(np, func(i, j int) bool {
		return np[i].Name < np[j].Name
	})

	return np, nil
}

func (api *API) blueprintsFreezeHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type blueprintFrozen struct {
		Blueprint blueprint.Blueprint `json:"blueprint"`
	}

	type reply struct {
		Blueprints []blueprintFrozen `json:"blueprints"`
		Errors     []responseError   `json:"errors"`
	}

	names := strings.Split(params.ByName("blueprints"), ",")
	if names[0] == "/" {
		errors := responseError{
			Code: http.StatusNotFound,
			ID:   "HTTPError",
			Msg:  "Not Found",
		}
		statusResponseError(writer, http.StatusNotFound, errors)
		return
	}

	// Remove the leading / from the first entry (check above ensures it is not just a /
	names[0] = names[0][1:]

	if !verifyStringsWithRegex(writer, names, ValidBlueprintName) {
		return
	}

	blueprints := []blueprintFrozen{}
	errors := []responseError{}
	for _, name := range names {
		bp, _ := api.store.GetBlueprint(name)
		if bp == nil {
			rerr := responseError{
				ID:  "UnknownBlueprint",
				Msg: fmt.Sprintf("%s: blueprint not found", name),
			}
			errors = append(errors, rerr)
			break
		}
		// Make a copy of the blueprint since we will be replacing the version globs
		blueprint := bp.DeepCopy()
		dependencies, err := api.depsolveBlueprint(blueprint)
		if err != nil {
			rerr := responseError{
				ID:  "BlueprintsError",
				Msg: fmt.Sprintf("%s: %s", name, err.Error()),
			}
			errors = append(errors, rerr)
			break
		}
		// Sort dependencies by Name (names should be unique so no need to sort by EVRA)
		sort.Slice(dependencies, func(i, j int) bool {
			return dependencies[i].Name < dependencies[j].Name
		})

		// Expand any package name globs and set the package version
		newPackages, err := expandBlueprintGlobs(dependencies, blueprint.Packages)
		if err != nil {
			rerr := responseError{
				ID:  "BlueprintsError",
				Msg: fmt.Sprintf("%s: %s", name, err.Error()),
			}
			errors = append(errors, rerr)
			break
		}
		blueprint.Packages = newPackages

		// Expand any module name globs and set the module version
		newModules, err := expandBlueprintGlobs(dependencies, blueprint.Modules)
		if err != nil {
			rerr := responseError{
				ID:  "BlueprintsError",
				Msg: fmt.Sprintf("%s: %s", name, err.Error()),
			}
			errors = append(errors, rerr)
			break
		}
		blueprint.Modules = newModules
		blueprints = append(blueprints, blueprintFrozen{blueprint})
	}

	format := request.URL.Query().Get("format")

	if format == "toml" {
		// lorax concatenates multiple blueprints with `\n\n` here,
		// which is never useful. Deviate by only returning the first
		// blueprint.
		if len(blueprints) == 0 {
			// lorax-composer just outputs an empty string if there were no blueprints
			writer.WriteHeader(http.StatusOK)
			fmt.Fprintf(writer, "")
			return
		} else if len(blueprints) > 1 {
			errors := responseError{
				ID:  "HTTPError",
				Msg: "toml format only supported when requesting one blueprint",
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}

		encoder := toml.NewEncoder(writer)
		encoder.Indent = ""
		err := encoder.Encode(blueprints[0].Blueprint)
		common.PanicOnError(err)
	} else {
		err := json.NewEncoder(writer).Encode(reply{
			Blueprints: blueprints,
			Errors:     errors,
		})
		common.PanicOnError(err)
	}
}

func (api *API) blueprintsDiffHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type pack struct {
		Package blueprint.Package `json:"Package"`
	}

	type diff struct {
		New *pack `json:"new"`
		Old *pack `json:"old"`
	}

	type reply struct {
		Diffs []diff `json:"diff"`
	}

	name := params.ByName("blueprint")
	if !verifyStringsWithRegex(writer, []string{name}, ValidBlueprintName) {
		return
	}

	fromCommit := params.ByName("from")
	if !verifyStringsWithRegex(writer, []string{fromCommit}, ValidBlueprintName) {
		return
	}

	toCommit := params.ByName("to")
	if !verifyStringsWithRegex(writer, []string{toCommit}, ValidBlueprintName) {
		return
	}

	if len(name) == 0 || len(fromCommit) == 0 || len(toCommit) == 0 {
		errors := responseError{
			Code: http.StatusNotFound,
			ID:   "HTTPError",
			Msg:  "Not Found",
		}
		statusResponseError(writer, http.StatusNotFound, errors)
		return
	}
	if fromCommit != "NEWEST" {
		errors := responseError{
			ID:  "UnknownCommit",
			Msg: fmt.Sprintf("ggit-error: revspec '%s' not found (-3)", fromCommit),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
	if toCommit != "WORKSPACE" {
		errors := responseError{
			ID:  "UnknownCommit",
			Msg: fmt.Sprintf("ggit-error: revspec '%s' not found (-3)", toCommit),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	// Fetch old and new blueprint details from store and return error if not found
	oldBlueprint := api.store.GetBlueprintCommitted(name)
	newBlueprint, _ := api.store.GetBlueprint(name)
	if oldBlueprint == nil || newBlueprint == nil {
		errors := responseError{
			ID:  "UnknownBlueprint",
			Msg: fmt.Sprintf("Unknown blueprint name: %s", name),
		}
		statusResponseError(writer, http.StatusNotFound, errors)
		return
	}

	newSlice := newBlueprint.Packages
	oldMap := make(map[string]blueprint.Package)
	diffs := []diff{}

	for _, oldPackage := range oldBlueprint.Packages {
		oldMap[oldPackage.Name] = oldPackage
	}

	// For each package in new blueprint check if the old one contains it
	for _, newPackage := range newSlice {
		oldPackage, found := oldMap[newPackage.Name]
		// If found remove from old packages map but otherwise create a diff with the added package
		if found {
			delete(oldMap, oldPackage.Name)
			// Create a diff if the versions changed
			if oldPackage.Version != newPackage.Version {
				diffs = append(diffs, diff{Old: &pack{oldPackage}, New: &pack{newPackage}})
			}
		} else {
			diffs = append(diffs, diff{Old: nil, New: &pack{newPackage}})
		}
	}

	// All packages remaining in the old packages map have been removed in the new blueprint so create a diff
	for _, oldPackage := range oldMap {
		diffs = append(diffs, diff{Old: &pack{oldPackage}, New: nil})
	}

	err := json.NewEncoder(writer).Encode(reply{diffs})
	common.PanicOnError(err)
}

// blueprintsChangeHandler returns a specific change to a blueprint
func (api *API) blueprintsChangeHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 1) {
		return
	}
	name := params.ByName("blueprint")
	if !verifyStringsWithRegex(writer, []string{name}, ValidBlueprintName) {
		return
	}

	commit := params.ByName("commit")
	if !verifyStringsWithRegex(writer, []string{commit}, ValidBlueprintName) {
		return
	}

	bpChange, err := api.store.GetBlueprintChange(name, commit)
	if err != nil {
		errors := responseError{
			ID:  "UnknownCommit",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	bp := bpChange.Blueprint
	if len(bpChange.Blueprint.Name) == 0 {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: fmt.Sprintf("no blueprint found for commit %s", commit),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		errors := responseError{
			ID:  "InvalidChars",
			Msg: fmt.Sprintf("invalid query string: %v", err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	format := query.Get("format")
	if format == "json" || format == "" {
		err := json.NewEncoder(writer).Encode(bp)
		common.PanicOnError(err)
	} else if format == "toml" {
		encoder := toml.NewEncoder(writer)
		encoder.Indent = ""
		err := encoder.Encode(bp)
		common.PanicOnError(err)
	} else {
		errors := responseError{
			ID:  "InvalidChars",
			Msg: fmt.Sprintf("invalid format parameter: %s", format),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
}

func (api *API) blueprintsChangesHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type change struct {
		Changes []blueprint.Change `json:"changes"`
		Name    string             `json:"name"`
		Total   int                `json:"total"`
	}

	type reply struct {
		BlueprintsChanges []change        `json:"blueprints"`
		Errors            []responseError `json:"errors"`
		Limit             uint            `json:"limit"`
		Offset            uint            `json:"offset"`
	}

	names := strings.Split(params.ByName("blueprints"), ",")
	if names[0] == "/" {
		errors := responseError{
			Code: http.StatusNotFound,
			ID:   "HTTPError",
			Msg:  "Not Found",
		}
		statusResponseError(writer, http.StatusNotFound, errors)
		return
	}

	// Remove the leading / from the first entry (check above ensures it is not just a /
	names[0] = names[0][1:]

	if !verifyStringsWithRegex(writer, names, ValidBlueprintName) {
		return
	}

	offset, limit, err := parseOffsetAndLimit(request.URL.Query())
	if err != nil {
		errors := responseError{
			ID:  "BadLimitOrOffset",
			Msg: fmt.Sprintf("BadRequest: %s", err.Error()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	allChanges := []change{}
	errors := []responseError{}
	for _, name := range names {
		bpChanges := api.store.GetBlueprintChanges(name)
		// Reverse the changes, newest first
		reversed := make([]blueprint.Change, 0, len(bpChanges))
		for i := len(bpChanges) - 1; i >= 0; i-- {
			reversed = append(reversed, bpChanges[i])
		}
		if bpChanges != nil {
			change := change{
				Changes: reversed,
				Name:    name,
				Total:   len(bpChanges),
			}
			allChanges = append(allChanges, change)
		} else {
			error := responseError{
				ID:  "UnknownBlueprint",
				Msg: name,
			}
			errors = append(errors, error)
		}
	}

	err = json.NewEncoder(writer).Encode(reply{
		BlueprintsChanges: allChanges,
		Errors:            errors,
		Offset:            offset,
		Limit:             limit,
	})
	common.PanicOnError(err)
}

func (api *API) blueprintsNewHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	contentType := request.Header["Content-Type"]
	if len(contentType) == 0 {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: "missing Content-Type header",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	if request.ContentLength == 0 {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: "Missing blueprint",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	var blueprint blueprint.Blueprint
	var err error
	if contentType[0] == "application/json" {
		err = json.NewDecoder(request.Body).Decode(&blueprint)
	} else if contentType[0] == "text/x-toml" {
		_, err = toml.NewDecoder(request.Body).Decode(&blueprint)
	} else {
		err = errors.New("blueprint must be in json or toml format")
	}

	if err != nil {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: "400 Bad Request: The browser (or proxy) sent a request that this server could not understand: " + err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	if !verifyStringsWithRegex(writer, []string{blueprint.Name}, ValidBlueprintName) {
		return
	}

	// Check the blueprint's distro to make sure it is valid
	if len(blueprint.Distro) > 0 {
		// NB: For backward compatibility, try to to standardize the distro name,
		// because it may be missing a dot to separate major and minor verion.
		// If it fails, just use the original name.
		if distroStandardized, err := distroidparser.DefaultParser.Standardize(blueprint.Distro); err == nil {
			blueprint.Distro = distroStandardized
		}

		arch := blueprint.Arch
		if arch == "" {
			arch = api.hostArch
		}
		if !common.IsStringInSortedSlice(api.validDistros(arch), blueprint.Distro) {
			errors := responseError{
				ID:  "BlueprintsError",
				Msg: fmt.Sprintf("'%s' is not a valid distribution (architecture '%s')", blueprint.Distro, arch),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}
	}

	// Make sure the blueprint has default values and that the version is valid
	err = blueprint.Initialize()
	if err != nil {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	commitMsg := "Recipe " + blueprint.Name + ", version " + blueprint.Version + " saved."
	err = api.store.PushBlueprint(blueprint, commitMsg)
	if err != nil {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	statusResponseOK(writer)
}

func (api *API) blueprintsWorkspaceHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	contentType := request.Header["Content-Type"]
	if len(contentType) == 0 {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: "missing Content-Type header",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	if request.ContentLength == 0 {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: "Missing blueprint",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	var blueprint blueprint.Blueprint
	var err error
	if contentType[0] == "application/json" {
		err = json.NewDecoder(request.Body).Decode(&blueprint)
	} else if contentType[0] == "text/x-toml" {
		_, err = toml.NewDecoder(request.Body).Decode(&blueprint)
	} else {
		err = errors.New("blueprint must be in json or toml format")
	}

	if err != nil {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: "400 Bad Request: The browser (or proxy) sent a request that this server could not understand: " + err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	if !verifyStringsWithRegex(writer, []string{blueprint.Name}, ValidBlueprintName) {
		return
	}

	err = api.store.PushBlueprintToWorkspace(blueprint)
	if err != nil {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	statusResponseOK(writer)
}

func (api *API) blueprintUndoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	name := params.ByName("blueprint")
	if !verifyStringsWithRegex(writer, []string{name}, ValidBlueprintName) {
		return
	}

	commit := params.ByName("commit")
	if !verifyStringsWithRegex(writer, []string{commit}, ValidBlueprintName) {
		return
	}

	bpChange, err := api.store.GetBlueprintChange(name, commit)
	if err != nil {
		errors := responseError{
			ID:  "UnknownCommit",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	bp := bpChange.Blueprint
	if len(bpChange.Blueprint.Name) == 0 {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: fmt.Sprintf("no blueprint found for commit %s", commit),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	commitMsg := name + ".toml reverted to commit " + commit
	err = api.store.PushBlueprint(bp, commitMsg)
	if err != nil {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
	statusResponseOK(writer)
}

func (api *API) blueprintDeleteHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	name := params.ByName("blueprint")
	if !verifyStringsWithRegex(writer, []string{name}, ValidBlueprintName) {
		return
	}

	if err := api.store.DeleteBlueprint(name); err != nil {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
	statusResponseOK(writer)
}

func (api *API) blueprintDeleteWorkspaceHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	name := params.ByName("blueprint")
	if !verifyStringsWithRegex(writer, []string{name}, ValidBlueprintName) {
		return
	}

	if err := api.store.DeleteBlueprintFromWorkspace(name); err != nil {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
	statusResponseOK(writer)
}

// blueprintsTagHandler tags the current blueprint commit as a new revision
func (api *API) blueprintsTagHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	name := params.ByName("blueprint")
	if !verifyStringsWithRegex(writer, []string{name}, ValidBlueprintName) {
		return
	}

	err := api.store.TagBlueprint(name)
	if err != nil {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
	statusResponseOK(writer)
}

// depsolve handles depsolving package sets required for serializing a manifest for a given distribution.
//
// Distro name is not determined from the provided architecture, but has to be provided explicitly.
// The reason is that a distro name alias may have been used to get the distro object as well as the
// repositories for the depsolving. The actual distro object name may not correspond to the alias.
// Since the solver uses the distro name to namespace cache, it is important to use the same distro
// name as the one used to get the repositories.
func (api *API) depsolve(packageSets map[string][]rpmmd.PackageSet, distroName string, arch distro.Arch) (map[string]depsolvednf.DepsolveResult, error) {

	distro := arch.Distro()
	platformID := distro.ModulePlatformID()
	releasever := distro.Releasever()
	solver := api.getSolver(platformID, releasever, arch.Name(), distroName)

	depsolvedSets := make(map[string]depsolvednf.DepsolveResult, len(packageSets))

	for name, pkgSet := range packageSets {
		res, err := solver.Depsolve(pkgSet, sbom.StandardTypeNone)
		if err != nil {
			return nil, err
		}
		depsolvedSets[name] = *res
	}
	if err := solver.CleanCache(); err != nil {
		// log and ignore
		log.Printf("Error during rpm repo cache cleanup: %s", err.Error())
	}
	return depsolvedSets, nil
}

func (api *API) resolveContainers(sourceSpecs map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error) {

	specs := make(map[string][]container.Spec, len(sourceSpecs))

	// shortcut
	if len(sourceSpecs) == 0 {
		return specs, nil
	}

	// Run one job for each value in the sourceSpecs in order.
	// Currently this should still only be one job, but if containers are added
	// to multiple pipelines at any point, this should work.
	for name, sources := range sourceSpecs {
		job := worker.ContainerResolveJob{
			Arch:  archName,
			Specs: make([]worker.ContainerSpec, len(sources)),
		}

		for i, c := range sources {
			job.Specs[i] = worker.ContainerSpec{
				Source:    c.Source,
				Name:      c.Name,
				TLSVerify: c.TLSVerify,
			}
		}

		jobId, err := api.workers.EnqueueContainerResolveJob(&job, "")

		if err != nil {
			return specs, err
		}

		var result worker.ContainerResolveJobResult

		for {
			jobInfo, err := api.workers.ContainerResolveJobInfo(jobId, &result)

			if err != nil {
				return specs, err
			}

			if result.JobError != nil {
				return specs, errors.New(result.JobError.Reason)
			} else if jobInfo.JobStatus.Canceled {
				return specs, fmt.Errorf("Failed to resolve containers: job cancelled")
			} else if !jobInfo.JobStatus.Finished.IsZero() {
				break
			}

			time.Sleep(time.Millisecond * 250)
		}

		if len(result.Specs) != len(sources) {
			panic("programming error: input / output length don't match")
		}

		specs[name] = make([]container.Spec, len(sources))
		for i, s := range result.Specs {
			specs[name][i].Source = s.Source
			specs[name][i].Digest = s.Digest
			specs[name][i].LocalName = s.Name
			specs[name][i].TLSVerify = s.TLSVerify
			specs[name][i].ImageID = s.ImageID
			specs[name][i].ListDigest = s.ListDigest
		}
	}

	return specs, nil
}

func (api *API) resolveOSTreeCommits(sourceSpecs map[string][]ostree.SourceSpec, test bool) (map[string][]ostree.CommitSpec, error) {
	commitSpecs := make(map[string][]ostree.CommitSpec, len(sourceSpecs))
	for name, sources := range sourceSpecs {
		commits := make([]ostree.CommitSpec, len(sources))
		for idx, source := range sources {
			if test {
				checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(source.URL+source.Ref)))
				commits[idx] = ostree.CommitSpec{
					Ref:      source.Ref,
					URL:      source.URL,
					Checksum: checksum,
				}
			} else {
				// MTLS not supported on-prem
				commit, err := ostree.Resolve(source)
				if err != nil {
					return nil, err
				}
				commits[idx] = commit
			}
		}
		commitSpecs[name] = commits
	}
	return commitSpecs, nil
}

// Schedule new compose by first translating the appropriate blueprint into a pipeline and then
// pushing it into the channel for waiting builds.
func (api *API) composeHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	// https://weldr.io/lorax/pylorax.api.html#pylorax.api.v0.v0_compose_start
	type ComposeRequest struct {
		BlueprintName string               `json:"blueprint_name"`
		ComposeType   string               `json:"compose_type"`
		Size          uint64               `json:"size"`
		OSTree        *ostree.ImageOptions `json:"ostree,omitempty"`
		Branch        string               `json:"branch"`
		Upload        *uploadRequest       `json:"upload"`
	}
	type ComposeReply struct {
		BuildID  uuid.UUID `json:"build_id"`
		Status   bool      `json:"status"`
		Warnings []string  `json:"warnings"`
	}

	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		errors := responseError{
			ID:  "MissingPost",
			Msg: "blueprint must be json",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	var cr ComposeRequest
	err := json.NewDecoder(request.Body).Decode(&cr)
	if err != nil {
		errors := responseError{
			Code: http.StatusNotFound,
			ID:   "HTTPError",
			Msg:  "Not Found",
		}
		statusResponseError(writer, http.StatusNotFound, errors)
		return
	}

	if !verifyStringsWithRegex(writer, []string{cr.BlueprintName}, ValidBlueprintName) {
		return
	}

	bp := api.store.GetBlueprintCommitted(cr.BlueprintName)
	if bp == nil {
		errors := responseError{
			ID:  "UnknownBlueprint",
			Msg: fmt.Sprintf("Unknown blueprint name: %s", cr.BlueprintName),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	distroName := bp.Distro
	if distroName == "" {
		distroName = api.hostDistroName
	}

	archName := bp.Arch
	if archName == "" {
		archName = api.hostArch
	}

	if api.getDistro(distroName, archName) == nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: fmt.Sprintf("Unknown distribution: %s for arch %s", distroName, archName),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	// Get the imageType that corresponds to the distribution selected by the blueprint
	imageType, err := api.getImageType(distroName, cr.ComposeType, archName)
	if err != nil {
		errors := responseError{
			ID:  "ComposeError",
			Msg: fmt.Sprintf("Failed to get compose type %q: %v", cr.ComposeType, err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	composeID := uuid.New()

	var targets []*target.Target
	// Always instruct the worker to upload the artifact back to the server
	workerServerTarget := target.NewWorkerServerTarget()
	workerServerTarget.ImageName = imageType.Filename()
	workerServerTarget.OsbuildArtifact.ExportFilename = imageType.Filename()
	workerServerTarget.OsbuildArtifact.ExportName = imageType.Exports()[0]
	targets = append(targets, workerServerTarget)
	if isRequestVersionAtLeast(params, 1) && cr.Upload != nil {
		t := uploadRequestToTarget(*cr.Upload, imageType)
		targets = append(targets, t)
	}

	// Check for test parameter
	q, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		errors := responseError{
			ID:  "InvalidChars",
			Msg: fmt.Sprintf("invalid query string: %v", err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	var size uint64

	// check if filesytem customizations have been set.
	// if compose size parameter is set, take the larger of
	// the two values
	if minSize := bp.Customizations.GetFilesystemsMinSize(); bp.Customizations != nil && minSize > 0 && minSize > cr.Size {
		size = imageType.Size(minSize)
	} else {
		size = imageType.Size(cr.Size)
	}

	bigSeed, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		panic("cannot generate a manifest seed: " + err.Error())
	}
	seed := bigSeed.Int64()

	// Get the partitioning mode
	pm, err := bp.Customizations.GetPartitioningMode()
	if err != nil {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	options := distro.ImageOptions{
		Size:             size,
		OSTree:           cr.OSTree,
		PartitioningMode: partition.PartitioningMode(pm),
	}
	options.Facts = &facts.ImageOptions{
		APIType: facts.WELDR_APITYPE,
	}

	imageRepos, err := api.allRepositoriesByImageType(distroName, imageType)
	if err != nil {
		errors := responseError{
			ID:  "InternalError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}

	manifest, warnings, err := imageType.Manifest(bp, options, imageRepos, &seed)
	if err != nil {
		errors := responseError{
			ID:  "ManifestCreationFailed",
			Msg: fmt.Sprintf("failed to initialize osbuild manifest: %v", err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	depsolved, err := api.depsolve(manifest.GetPackageSetChains(), distroName, imageType.Arch())
	if err != nil {
		errors := responseError{
			ID:  "DepsolveError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}

	containerSpecs, err := api.resolveContainers(manifest.GetContainerSourceSpecs(), archName)
	if err != nil {
		errors := responseError{
			ID:  "ContainerResolveError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}

	testMode := q.Get("test")

	ostreeCommitSpecs, err := api.resolveOSTreeCommits(manifest.GetOSTreeSourceSpecs(), testMode == "1" || testMode == "2")
	if err != nil {
		errors := responseError{
			ID:  "OSTreeOptionsError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	mf, err := manifest.Serialize(depsolved, containerSpecs, ostreeCommitSpecs, nil)
	if err != nil {
		errors := responseError{
			ID:  "ManifestCreationFailed",
			Msg: fmt.Sprintf("failed to serialize osbuild manifest: %v", err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	var packages []rpmmd.PackageSpec
	// TODO: introduce a way to query these from the manifest / image type
	// BUG: installer/container image types will have empty package sets
	if packages = depsolved["packages"].Packages; len(packages) == 0 {
		if packages = depsolved["os"].Packages; len(packages) == 0 {
			packages = depsolved["ostree-tree"].Packages
		}
	}

	workerAvailable, err := api.workers.WorkerAvailableForArch(archName)
	if err != nil {
		log.Println("error when pushing new compose: ", err.Error())
		errors := responseError{
			ID:  "ComposePushErrored",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}
	if !workerAvailable {
		errors := responseError{
			ID:  "ComposePushErrored",
			Msg: fmt.Sprintf("No worker for arch '%s'  available", archName),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	if testMode == "1" {
		// Create a failed compose
		err = api.store.PushTestCompose(composeID, mf, imageType, bp, size, targets, false, packages)
	} else if testMode == "2" {
		// Create a successful compose
		err = api.store.PushTestCompose(composeID, mf, imageType, bp, size, targets, true, packages)
	} else {
		var jobId uuid.UUID
		jobId, err = api.workers.EnqueueOSBuild(archName, &worker.OSBuildJob{
			Manifest: mf,
			Targets:  targets,
			PipelineNames: &worker.PipelineNames{
				Build:   manifest.BuildPipelines(),
				Payload: manifest.PayloadPipelines(),
			},
			ImageBootMode: imageType.BootMode().String(),
		}, "")
		if err == nil {
			err = api.store.PushCompose(composeID, mf, imageType, bp, size, targets, jobId, packages)
		}
	}

	// TODO: we should probably do some kind of blueprint validation in future
	// for now, let's just 500 and bail out
	if err != nil {
		log.Println("error when pushing new compose: ", err.Error())
		errors := responseError{
			ID:  "ComposePushErrored",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}

	err = json.NewEncoder(writer).Encode(ComposeReply{
		BuildID:  composeID,
		Status:   true,
		Warnings: warnings,
	})
	common.PanicOnError(err)
}

func (api *API) composeDeleteHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type composeDeleteStatus struct {
		UUID   uuid.UUID `json:"uuid"`
		Status bool      `json:"status"`
	}

	type composeDeleteError struct {
		ID  string `json:"id"`
		Msg string `json:"msg"`
	}

	uuidsParam := params.ByName("uuids")

	results := []composeDeleteStatus{}
	errors := []composeDeleteError{}
	uuidStrings := strings.Split(uuidsParam, ",")
	for _, uuidString := range uuidStrings {
		id, err := uuid.Parse(uuidString)
		if err != nil {
			errors = append(errors, composeDeleteError{
				"UnknownUUID",
				fmt.Sprintf("%s is not a valid uuid", uuidString),
			})
			continue
		}

		compose, exists := api.store.GetCompose(id)
		if !exists {
			errors = append(errors, composeDeleteError{
				"UnknownUUID",
				fmt.Sprintf("compose %s doesn't exist", id),
			})
			continue
		}

		composeStatus, err := api.getComposeStatus(compose)
		if err != nil {
			errors = append(errors, composeDeleteError{
				"ComposeStatusError",
				fmt.Sprintf("Error getting status of compose %s: %s", id, err),
			})
			continue
		}
		if composeStatus.State != ComposeFinished && composeStatus.State != ComposeFailed {
			errors = append(errors, composeDeleteError{
				"BuildInWrongState",
				fmt.Sprintf("Compose %s is not in FINISHED or FAILED.", id),
			})
			continue
		}

		err = api.store.DeleteCompose(id)
		if err != nil {
			errors = append(errors, composeDeleteError{
				"ComposeError",
				fmt.Sprintf("%s: %s", id, err.Error()),
			})
			continue
		}

		// Delete artifacts from the worker server or — if that doesn't
		// have this job — the compat output dir. Ignore errors,
		// because there's no point of reporting them to the client
		// after the compose itself has already been deleted.
		err = api.workers.DeleteArtifacts(compose.ImageBuild.JobID)
		if err == jobqueue.ErrNotExist && api.compatOutputDir != "" {
			_ = os.RemoveAll(path.Join(api.compatOutputDir, id.String()))
		}

		results = append(results, composeDeleteStatus{id, true})
	}

	reply := struct {
		UUIDs  []composeDeleteStatus `json:"uuids"`
		Errors []composeDeleteError  `json:"errors"`
	}{results, errors}

	err := json.NewEncoder(writer).Encode(reply)
	common.PanicOnError(err)
}

func (api *API) composeCancelHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	uuidString := params.ByName("uuid")
	id, err := uuid.Parse(uuidString)
	if err != nil {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("%s is not a valid build uuid", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	compose, exists := api.store.GetCompose(id)
	if !exists {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("Compose %s doesn't exist", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	composeStatus, err := api.getComposeStatus(compose)
	if err != nil {
		errors := responseError{
			ID:  "ComposeStatusError",
			Msg: fmt.Sprintf("Error getting status of compose %s: %s", id, err),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}
	if composeStatus.State != ComposeWaiting && composeStatus.State != ComposeRunning {
		errors := responseError{
			ID:  "BuildInWrongState",
			Msg: fmt.Sprintf("Build %s is not in WAITING or RUNNING.", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	err = api.workers.Cancel(compose.ImageBuild.JobID)
	if err != nil {
		errors := responseError{
			ID:  "InternalServerError",
			Msg: fmt.Sprintf("Internal server error: %v", err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	reply := CancelComposeStatusV0{id, true}
	_ = json.NewEncoder(writer).Encode(reply)
}

func (api *API) composeTypesHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	archName := api.hostArch
	if queryArch := request.URL.Query().Get("arch"); queryArch != "" {
		archName = queryArch
	}

	// Optional distro parameter
	// If it is empty it will return api.hostDistroName
	distroName, err := api.parseDistro(request.URL.Query(), archName)
	if err != nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	d := api.getDistro(distroName, archName)
	if d == nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: fmt.Sprintf("Unknown distribution: %s", distroName),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	// Get the distro specific arch so that we can return the correct list of image types
	arch, err := d.GetArch(archName)
	if err != nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: fmt.Sprintf("Unknown arch: %s", arch),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	type composeType struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}

	var reply struct {
		Types []composeType `json:"types"`
	}

	for _, format := range arch.ListImageTypes() {
		imgAllowed, err := api.isImageTypeAllowed(distroName, format)
		if err != nil {
			errors := responseError{
				ID:  "InternalError",
				Msg: fmt.Sprintf("Error while checking if image type is allowed: %v", err),
			}
			statusResponseError(writer, http.StatusInternalServerError, errors)
			return
		}
		if !imgAllowed {
			continue
		}
		reply.Types = append(reply.Types, composeType{format, true})
	}

	err = json.NewEncoder(writer).Encode(reply)
	common.PanicOnError(err)
}

func (api *API) composeQueueHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	reply := struct {
		New []*ComposeEntry `json:"new"`
		Run []*ComposeEntry `json:"run"`
	}{[]*ComposeEntry{}, []*ComposeEntry{}}

	includeUploads := isRequestVersionAtLeast(params, 1)

	composes := api.store.GetAllComposes()
	for id, compose := range composes {
		composeStatus, err := api.getComposeStatus(compose)
		if err != nil {
			log.Printf("Error getting status of compose %s: %s", id, err)
			continue
		}
		switch composeStatus.State {
		case ComposeWaiting:
			reply.New = append(reply.New, composeToComposeEntry(id, compose, composeStatus, includeUploads))
		case ComposeRunning:
			reply.Run = append(reply.Run, composeToComposeEntry(id, compose, composeStatus, includeUploads))
		}
	}

	err := json.NewEncoder(writer).Encode(reply)
	common.PanicOnError(err)
}

func (api *API) composeStatusHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	// TODO: lorax has some params: /api/v0/compose/status/<uuids>[?blueprint=<blueprint_name>&status=<compose_status>&type=<compose_type>]
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	var reply struct {
		UUIDs []*ComposeEntry `json:"uuids"`
	}

	uuidsParam := params.ByName("uuids")

	composes := api.store.GetAllComposes()
	uuids := []uuid.UUID{}

	if uuidsParam != "*" {
		for _, uuidString := range strings.Split(uuidsParam, ",") {
			id, err := uuid.Parse(uuidString)
			if err != nil {
				errors := responseError{
					ID:  "UnknownUUID",
					Msg: fmt.Sprintf("%s is not a valid build uuid", uuidString),
				}
				statusResponseError(writer, http.StatusBadRequest, errors)
				return
			}
			uuids = append(uuids, id)
		}
	} else {
		for id := range composes {
			uuids = append(uuids, id)
		}
	}

	q, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		errors := responseError{
			ID:  "InvalidChars",
			Msg: fmt.Sprintf("invalid query string: %v", err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	filterBlueprint := q.Get("blueprint")
	if len(filterBlueprint) > 0 && !verifyStringsWithRegex(writer, []string{filterBlueprint}, ValidBlueprintName) {
		return
	}

	filterStatus := q.Get("status")
	filterImageType := q.Get("type")

	filteredUUIDs := []uuid.UUID{}
	for _, id := range uuids {
		compose, exists := composes[id]
		if !exists {
			continue
		}
		composeStatus, err := api.getComposeStatus(compose)
		if err != nil {
			log.Printf("Error getting status of compose %s: %s", id, err)
			continue
		}

		if filterBlueprint != "" && compose.Blueprint.Name != filterBlueprint {
			continue
		} else if filterStatus != "" && composeStatus.State.ToString() != filterStatus {
			continue
		} else if filterImageType != "" && compose.ImageBuild.ImageType.Name() != filterImageType {
			continue
		}
		filteredUUIDs = append(filteredUUIDs, id)
	}

	reply.UUIDs = []*ComposeEntry{}
	includeUploads := isRequestVersionAtLeast(params, 1)
	for _, id := range filteredUUIDs {
		if compose, exists := composes[id]; exists {
			composeStatus, err := api.getComposeStatus(compose)
			if err != nil {
				log.Printf("Error getting status of compose %s: %s", id, err)
				continue
			}

			reply.UUIDs = append(reply.UUIDs, composeToComposeEntry(id, compose, composeStatus, includeUploads))
		}
	}
	sortComposeEntries(reply.UUIDs)

	err = json.NewEncoder(writer).Encode(reply)
	common.PanicOnError(err)
}

func (api *API) composeInfoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	uuidString := params.ByName("uuid")
	id, err := uuid.Parse(uuidString)
	if err != nil {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("%s is not a valid build uuid", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	compose, exists := api.store.GetCompose(id)

	if !exists {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("%s is not a valid build uuid", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	var reply struct {
		ID        uuid.UUID            `json:"id"`
		Config    string               `json:"config"`    // anaconda config, let's ignore this field
		Blueprint *blueprint.Blueprint `json:"blueprint"` // blueprint not frozen!
		Commit    string               `json:"commit"`    // empty for now
		Deps      struct {
			Packages []rpmmd.PackageSpec `json:"packages"`
		} `json:"deps"`
		ComposeType string           `json:"compose_type"`
		QueueStatus string           `json:"queue_status"`
		ImageSize   uint64           `json:"image_size"`
		Uploads     []uploadResponse `json:"uploads,omitempty"`
	}

	reply.ID = id
	reply.Blueprint = compose.Blueprint
	// Weldr API assumes only one image build per compose, that's why only the
	// 1st build is considered
	composeStatus, err := api.getComposeStatus(compose)
	if err != nil {
		errors := responseError{
			ID:  "ComposeStatusError",
			Msg: fmt.Sprintf("Error getting status of compose %s: %s", id, err),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}

	reply.ComposeType = compose.ImageBuild.ImageType.Name()
	reply.QueueStatus = composeStatus.State.ToString()
	reply.ImageSize = compose.ImageBuild.Size

	if isRequestVersionAtLeast(params, 1) {
		reply.Uploads = targetsToUploadResponses(compose.ImageBuild.Targets, composeStatus.State)
	}

	// Add package dependencies from the compose
	// This information may not be included for the compose
	dependencies := compose.Packages
	// Sort dependencies by Name (names should be unique so no need to sort by EVRA)
	sort.Slice(dependencies, func(i, j int) bool {
		return dependencies[i].Name < dependencies[j].Name
	})
	reply.Deps.Packages = dependencies
	err = json.NewEncoder(writer).Encode(reply)
	common.PanicOnError(err)
}

func (api *API) composeImageHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	uuidString := params.ByName("uuid")
	uuid, err := uuid.Parse(uuidString)
	if err != nil {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("%s is not a valid build uuid", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	compose, exists := api.store.GetCompose(uuid)
	if !exists {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("Compose %s doesn't exist", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	composeStatus, err := api.getComposeStatus(compose)
	if err != nil {
		errors := responseError{
			ID:  "ComposeStatusError",
			Msg: fmt.Sprintf("Error getting status of compose %s: %s", uuid, err),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}
	if composeStatus.State != ComposeFinished {
		errors := responseError{
			ID:  "BuildInWrongState",
			Msg: fmt.Sprintf("Build %s is in wrong state: %s", uuidString, composeStatus.State.ToString()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	imageName := compose.ImageBuild.ImageType.Filename()
	imageMime := compose.ImageBuild.ImageType.MIMEType()

	reader, fileSize, err := api.openImageFile(uuid, compose)
	if err != nil {
		errors := responseError{
			ID:  "InternalServerError",
			Msg: fmt.Sprintf("Error accessing image file for compose %s: %v", uuid, err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	writer.Header().Set("Content-Disposition", "attachment; filename="+uuid.String()+"-"+imageName)
	writer.Header().Set("Content-Type", imageMime)
	writer.Header().Set("Content-Length", fmt.Sprintf("%d", fileSize))

	_, err = io.Copy(writer, reader)
	common.PanicOnError(err)
}

// composeMetadataHandler returns a tar of the metadata used to compose the requested UUID
func (api *API) composeMetadataHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	uuidString := params.ByName("uuid")
	uuid, err := uuid.Parse(uuidString)
	if err != nil {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("%s is not a valid build uuid", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	compose, exists := api.store.GetCompose(uuid)
	if !exists {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("Compose %s doesn't exist", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	composeStatus, err := api.getComposeStatus(compose)
	if err != nil {
		errors := responseError{
			ID:  "ComposeStatusError",
			Msg: fmt.Sprintf("Error getting status of compose %s: %s", uuid, err),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}
	if composeStatus.State != ComposeFinished && composeStatus.State != ComposeFailed {
		errors := responseError{
			ID:  "BuildInWrongState",
			Msg: fmt.Sprintf("Build %s is in wrong state: %s", uuidString, composeStatus.State.ToString()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	metadata, err := json.Marshal(&compose.ImageBuild.Manifest)
	common.PanicOnError(err)

	writer.Header().Set("Content-Disposition", "attachment; filename="+uuid.String()+"-metadata.tar")
	writer.Header().Set("Content-Type", "application/x-tar")
	// NOTE: Do not set Content-Length, it will use chunked transfer encoding automatically

	tw := tar.NewWriter(writer)
	hdr := &tar.Header{
		Name:    uuid.String() + ".json",
		Mode:    0600,
		Size:    int64(len(metadata)),
		ModTime: time.Now().Truncate(time.Second),
	}
	err = tw.WriteHeader(hdr)
	common.PanicOnError(err)

	_, err = tw.Write(metadata)
	common.PanicOnError(err)

	err = tw.Close()
	common.PanicOnError(err)
}

// composeResultsHandler returns a tar of the metadata, logs, and image from a compose
func (api *API) composeResultsHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	uuidString := params.ByName("uuid")
	uuid, err := uuid.Parse(uuidString)
	if err != nil {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("%s is not a valid build uuid", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	compose, exists := api.store.GetCompose(uuid)
	if !exists {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("Compose %s doesn't exist", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	composeStatus, err := api.getComposeStatus(compose)
	if err != nil {
		errors := responseError{
			ID:  "ComposeStatusError",
			Msg: fmt.Sprintf("Error getting status of compose %s: %s", uuid, err),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}
	if composeStatus.State != ComposeFinished && composeStatus.State != ComposeFailed {
		errors := responseError{
			ID:  "BuildInWrongState",
			Msg: fmt.Sprintf("Build %s is in wrong state: %s", uuidString, composeStatus.State.ToString()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	metadata, err := json.Marshal(&compose.ImageBuild.Manifest)
	common.PanicOnError(err)

	writer.Header().Set("Content-Disposition", "attachment; filename="+uuid.String()+".tar")
	writer.Header().Set("Content-Type", "application/x-tar")
	// NOTE: Do not set Content-Length, it should use chunked transfer encoding automatically

	tw := tar.NewWriter(writer)
	hdr := &tar.Header{
		Name:    uuid.String() + ".json",
		Mode:    0644,
		Size:    int64(len(metadata)),
		ModTime: time.Now().Truncate(time.Second),
	}
	err = tw.WriteHeader(hdr)
	common.PanicOnError(err)
	_, err = tw.Write(metadata)
	common.PanicOnError(err)

	// Add the logs
	var fileContents bytes.Buffer
	if composeStatus.Result != nil {
		err = composeStatus.Result.Write(&fileContents)
		common.PanicOnError(err)

		hdr = &tar.Header{
			Name:    "logs/osbuild.log",
			Mode:    0644,
			Size:    int64(fileContents.Len()),
			ModTime: time.Now().Truncate(time.Second),
		}
		err = tw.WriteHeader(hdr)
		common.PanicOnError(err)
		_, err = tw.Write(fileContents.Bytes())
		common.PanicOnError(err)
	}

	reader, fileSize, err := api.openImageFile(uuid, compose)
	if err == nil {
		hdr = &tar.Header{
			Name:    uuid.String() + "-" + compose.ImageBuild.ImageType.Filename(),
			Mode:    0644,
			Size:    int64(fileSize),
			ModTime: time.Now().Truncate(time.Second),
		}
		err = tw.WriteHeader(hdr)
		common.PanicOnError(err)
		_, err = io.Copy(tw, reader)
		common.PanicOnError(err)
	}

	err = tw.Close()
	common.PanicOnError(err)
}

func (api *API) composeLogsHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	uuidString := params.ByName("uuid")
	id, err := uuid.Parse(uuidString)
	if err != nil {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("%s is not a valid build uuid", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	compose, exists := api.store.GetCompose(id)
	if !exists {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("Compose %s doesn't exist", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	composeStatus, err := api.getComposeStatus(compose)
	if err != nil {
		errors := responseError{
			ID:  "ComposeStatusError",
			Msg: fmt.Sprintf("Error getting status of compose %s: %s", id, err),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}
	if composeStatus.State != ComposeFinished && composeStatus.State != ComposeFailed {
		errors := responseError{
			ID:  "BuildInWrongState",
			Msg: fmt.Sprintf("Build %s not in FINISHED or FAILED state.", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	writer.Header().Set("Content-Disposition", "attachment; filename="+id.String()+"-logs.tar")
	writer.Header().Set("Content-Type", "application/x-tar")

	tw := tar.NewWriter(writer)

	// tar format needs to contain file size before the actual file content, therefore the intermediate buffer
	var fileContents bytes.Buffer
	err = composeStatus.Result.Write(&fileContents)
	common.PanicOnError(err)

	header := &tar.Header{
		Name:    "logs/osbuild.log",
		Mode:    0644,
		Size:    int64(fileContents.Len()),
		ModTime: time.Now().Truncate(time.Second),
	}

	err = tw.WriteHeader(header)
	common.PanicOnError(err)

	_, err = io.Copy(tw, &fileContents)
	common.PanicOnError(err)

	err = tw.Close()
	common.PanicOnError(err)
}

func (api *API) composeLogHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	// TODO: implement size param
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	uuidString := params.ByName("uuid")
	id, err := uuid.Parse(uuidString)
	if err != nil {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("%s is not a valid build uuid", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	compose, exists := api.store.GetCompose(id)
	if !exists {
		errors := responseError{
			ID:  "UnknownUUID",
			Msg: fmt.Sprintf("Compose %s doesn't exist", uuidString),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	composeStatus, err := api.getComposeStatus(compose)
	if err != nil {
		errors := responseError{
			ID:  "ComposeStatusError",
			Msg: fmt.Sprintf("Error getting status of compose %s: %s", id, err),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}
	if composeStatus.State == ComposeWaiting {
		errors := responseError{
			ID:  "BuildInWrongState",
			Msg: fmt.Sprintf("Build %s has not started yet. No logs to view.", uuidString),
		}
		statusResponseError(writer, http.StatusOK, errors) // weirdly, Lorax returns 200 in this case
		return
	}

	if composeStatus.State == ComposeRunning {
		fmt.Fprintf(writer, "Build %s is still running.\n", uuidString)
		return
	}

	err = composeStatus.Result.Write(writer)
	common.PanicOnError(err)
}

func (api *API) composeFinishedHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	reply := struct {
		Finished []*ComposeEntry `json:"finished"`
	}{[]*ComposeEntry{}}

	includeUploads := isRequestVersionAtLeast(params, 1)
	for id, compose := range api.store.GetAllComposes() {
		composeStatus, err := api.getComposeStatus(compose)
		if err != nil {
			log.Printf("Error getting status of compose %s: %s", id, err)
			continue
		}
		if composeStatus.State != ComposeFinished {
			continue
		}
		reply.Finished = append(reply.Finished, composeToComposeEntry(id, compose, composeStatus, includeUploads))
	}
	sortComposeEntries(reply.Finished)

	err := json.NewEncoder(writer).Encode(reply)
	common.PanicOnError(err)
}

func (api *API) composeFailedHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	reply := struct {
		Failed []*ComposeEntry `json:"failed"`
	}{[]*ComposeEntry{}}

	includeUploads := isRequestVersionAtLeast(params, 1)
	for id, compose := range api.store.GetAllComposes() {
		composeStatus, err := api.getComposeStatus(compose)
		if err != nil {
			log.Printf("Error getting status of compose %s: %s", id, err)
			continue
		}
		if composeStatus.State != ComposeFailed {
			continue
		}
		reply.Failed = append(reply.Failed, composeToComposeEntry(id, compose, composeStatus, includeUploads))
	}
	sortComposeEntries(reply.Failed)

	err := json.NewEncoder(writer).Encode(reply)
	common.PanicOnError(err)
}

// fetchPackageList returns the package list or the selected distribution
func (api *API) fetchPackageList(distroName, arch string, names []string) (packages rpmmd.PackageList, err error) {
	d := api.getDistro(distroName, arch)
	if d == nil {
		return nil, fmt.Errorf("GetDistro - unknown distribution: %s", distroName)
	}
	repos, err := api.allRepositories(distroName, arch)
	if err != nil {
		return nil, err
	}

	solver := api.getSolver(d.ModulePlatformID(), d.Releasever(), arch, d.Name())
	if len(names) == 0 {
		packages, err = solver.FetchMetadata(repos)
	} else {
		packages, err = solver.SearchMetadata(repos, names)
	}
	if err != nil {
		return nil, err
	}
	if err := solver.CleanCache(); err != nil {
		// log and ignore
		log.Printf("Error during rpm repo cache cleanup: %s", err.Error())
	}
	return packages, nil
}

// Returns only user-defined repositories, which should be used only for
// payload package sets.
func (api *API) payloadRepositories(distroName string) []rpmmd.RepoConfig {
	distroSourceConfigs := api.store.GetAllDistroSources(distroName)
	payloadRepos := make([]rpmmd.RepoConfig, 0, len(distroSourceConfigs))
	for id, source := range distroSourceConfigs {
		payloadRepos = append(payloadRepos, source.RepoConfig(id))
	}
	return payloadRepos
}

// Returns all configured repositories as rpmmd.RepoConfig.
// Payload repositories (defined by the user) are assigned the payload package
// set names from the image type.
//
// The difference from allRepositories() is that this method may return additional repositories
// which are needed to build the specific image type. The allRepositories() can't do this, because
// it is used in places where image types are not considered.
//
// Note that the distro name is not determined from the image type, but must be provided as an argument.
// This enables using distro name aliases to get repositories.
func (api *API) allRepositoriesByImageType(distroName string, imageType distro.ImageType) ([]rpmmd.RepoConfig, error) {
	repos, err := api.repoRegistry.ReposByImageTypeName(distroName, imageType.Arch().Name(), imageType.Name())
	if err != nil {
		return nil, err
	}

	payloadRepos := api.payloadRepositories(distroName)
	// tag payload repositories with the payload package set names and add them to list of repos
	for _, pr := range payloadRepos {
		pr.PackageSets = imageType.PayloadPackageSets()
		repos = append(repos, pr)
	}

	return repos, nil
}

// Returns all configured repositories (base + sources) as rpmmd.RepoConfig
func (api *API) allRepositories(distroName, arch string) ([]rpmmd.RepoConfig, error) {
	repos, err := api.repoRegistry.ReposByArchName(distroName, arch, false)
	if err != nil {
		return nil, err
	}

	payloadRepos := api.payloadRepositories(distroName)
	repos = append(repos, payloadRepos...)

	return repos, nil
}

func (api *API) depsolveBlueprint(bp blueprint.Blueprint) ([]rpmmd.PackageSpec, error) {
	// Depsolve using the host distro if none has been specified
	if bp.Distro == "" {
		bp.Distro = api.hostDistroName
	}

	arch := bp.Arch
	if arch == "" {
		arch = api.hostArch
	}

	d := api.getDistro(bp.Distro, arch)
	if d == nil {
		return nil, fmt.Errorf("GetDistro - unknown distribution: %s", bp.Distro)
	}
	repos, err := api.allRepositories(bp.Distro, arch)
	if err != nil {
		return nil, err
	}

	solver := api.getSolver(d.ModulePlatformID(), d.Releasever(), arch, d.Name())
	res, err := solver.Depsolve([]rpmmd.PackageSet{{Include: bp.GetPackages(), EnabledModules: bp.GetEnabledModules(), Repositories: repos}}, sbom.StandardTypeNone)
	if err != nil {
		return nil, err
	}

	if err := solver.CleanCache(); err != nil {
		// log and ignore
		log.Printf("Error during rpm repo cache cleanup: %s", err.Error())
	}
	return res.Packages, nil
}

func (api *API) uploadsScheduleHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 1) {
		return
	}

	// TODO: implement this route (it is v1 only)
	notImplementedHandler(writer, request, params)
}

func (api *API) uploadsDeleteHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 1) {
		return
	}

	// TODO: implement this route (it is v1 only)
	notImplementedHandler(writer, request, params)
}

func (api *API) uploadsInfoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 1) {
		return
	}

	// TODO: implement this route (it is v1 only)
	notImplementedHandler(writer, request, params)
}

func (api *API) uploadsLogHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 1) {
		return
	}

	// TODO: implement this route (it is v1 only)
	notImplementedHandler(writer, request, params)
}

func (api *API) uploadsResetHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 1) {
		return
	}

	// TODO: implement this route (it is v1 only)
	notImplementedHandler(writer, request, params)
}

func (api *API) uploadsCancelHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 1) {
		return
	}

	// TODO: implement this route (it is v1 only)
	notImplementedHandler(writer, request, params)
}

func (api *API) providersHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 1) {
		return
	}

	// TODO: implement this route (it is v1 only)
	notImplementedHandler(writer, request, params)
}

func (api *API) providersSaveHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 1) {
		return
	}

	// TODO: implement this route (it is v1 only)
	notImplementedHandler(writer, request, params)
}

func (api *API) providersDeleteHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 1) {
		return
	}

	// TODO: implement this route (it is v1 only)
	notImplementedHandler(writer, request, params)
}

func (api *API) distrosListHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 1) {
		return
	}

	archName := api.hostArch
	if arch := request.URL.Query().Get("arch"); arch != "" {
		archName = arch
	}

	var reply struct {
		Distros []string `json:"distros"`
	}
	reply.Distros = api.validDistros(archName)

	err := json.NewEncoder(writer).Encode(reply)
	common.PanicOnError(err)
}
