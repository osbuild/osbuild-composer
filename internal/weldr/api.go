package weldr

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	errors_package "errors"
	"fmt"
	"io"
	"io/ioutil"
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

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/reporegistry"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type API struct {
	store   *store.Store
	workers *worker.Server

	solver       *dnfjson.BaseSolver
	archName     string
	repoRegistry *reporegistry.RepoRegistry

	logger *log.Logger
	router *httprouter.Router
	server http.Server

	compatOutputDir string

	hostDistroName string                   // Name of the host distro
	distroRegistry *distroregistry.Registry // Available distros
	distros        []string                 // Supported distro names

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
	repos, err := api.repoRegistry.ReposByArchName(api.hostDistroName, api.archName, false)
	if err == nil {
		for _, repo := range repos {
			names = append(names, repo.Name)
		}
	}
	return names
}

// validDistros returns a list of distributions that also have repositories
func validDistros(rr *reporegistry.RepoRegistry, dr *distroregistry.Registry, arch string, logger *log.Logger) []string {
	distros := []string{}
	for _, d := range dr.List() {
		_, found := rr.DistroHasRepos(d, arch)
		if found {
			distros = append(distros, d)
		} else {
			logger.Printf("Distro %s has no repositories, skipping.", d)
		}
	}

	// NOTE: distro list is already sorted, so result will be sorted
	return distros
}

var ValidBlueprintName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// NewTestAPI is used for the test framework, sets up a single distro
func NewTestAPI(solver *dnfjson.BaseSolver, arch distro.Arch, dr *distroregistry.Registry,
	rr *reporegistry.RepoRegistry, logger *log.Logger,
	store *store.Store, workers *worker.Server, compatOutputDir string,
	distrosImageTypeDenylist map[string][]string) *API {

	// Use the first entry as the host distribution
	hostDistro := dr.GetDistro(dr.List()[0])
	api := &API{
		store:                    store,
		workers:                  workers,
		solver:                   solver,
		archName:                 arch.Name(),
		repoRegistry:             rr,
		logger:                   logger,
		compatOutputDir:          compatOutputDir,
		hostDistroName:           hostDistro.Name(),
		distroRegistry:           dr,
		distros:                  validDistros(rr, dr, arch.Name(), logger),
		distrosImageTypeDenylist: distrosImageTypeDenylist,
	}
	return setupRouter(api)
}

func New(repoPaths []string, stateDir string, solver *dnfjson.BaseSolver, dr *distroregistry.Registry,
	logger *log.Logger, workers *worker.Server, distrosImageTypeDenylist map[string][]string) (*API, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "", 0)
	}

	hostDistroName, _, _, err := common.GetHostDistroName()
	if err != nil {
		return nil, fmt.Errorf("failed to read host distro information")
	}
	archName := common.CurrentArch()

	rr, err := reporegistry.New(repoPaths)
	if err != nil {
		return nil, fmt.Errorf("error loading repository definitions: %v", err)
	}

	var hostArch distro.Arch
	hostDistro := dr.GetDistro(hostDistroName)
	if hostDistro != nil {
		// get canonical distro name if the host distro is supported
		hostDistroName = hostDistro.Name()

		hostArch, err = hostDistro.GetArch(archName)
		if err != nil {
			return nil, fmt.Errorf("Host distro does not support host architecture: %v", err)
		}

		// Check if repositories for the host distro and arch were loaded
		_, err = rr.ReposByArchName(hostDistroName, archName, false)
		if err != nil {
			log.Printf("loaded repository definitions don't contain any for the host distro/arch: %v", err)
		}

	} else {
		log.Printf("host distro %q is not supported: only cross-distro builds are available", hostDistroName)
	}

	store := store.New(&stateDir, hostArch, logger)
	compatOutputDir := path.Join(stateDir, "outputs")

	api := &API{
		store:                    store,
		workers:                  workers,
		solver:                   solver,
		archName:                 archName,
		repoRegistry:             rr,
		logger:                   logger,
		compatOutputDir:          compatOutputDir,
		hostDistroName:           hostDistroName,
		distroRegistry:           dr,
		distros:                  validDistros(rr, dr, archName, logger),
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
	api.server = http.Server{Handler: api}

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
func (api *API) getComposeStatus(compose store.Compose) *composeStatus {
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
		}
	}

	// All jobs are "osbuild" jobs.
	var result worker.OSBuildJobResult
	jobStatus, _, err := api.workers.OSBuildJobStatus(jobId, &result)
	if err != nil {
		panic(err)
	}

	return &composeStatus{
		State:    composeStateFromJobStatus(jobStatus, &result),
		Queued:   jobStatus.Queued,
		Started:  jobStatus.Started,
		Finished: jobStatus.Finished,
		Result:   result.OSBuildOutput,
	}
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
func (api *API) getImageType(distroName, imageType string) (distro.ImageType, error) {
	imgAllowed, err := api.isImageTypeAllowed(distroName, imageType)
	if err != nil {
		return nil, fmt.Errorf("error while checking if image type is allowed: %v", err)
	}
	if !imgAllowed {
		return nil, fmt.Errorf("image type %q for distro %q is denied by configuration", imageType, distroName)
	}

	distro := api.getDistro(distroName)
	if distro == nil {
		return nil, fmt.Errorf("GetDistro - unknown distribution: %s", distroName)
	}
	arch, err := distro.GetArch(api.archName)
	if err != nil {
		return nil, err
	}
	return arch.GetImageType(imageType)
}

func (api *API) parseDistro(query url.Values) (string, error) {
	if distro := query.Get("distro"); distro != "" {
		if common.IsStringInSortedSlice(api.distros, distro) {
			return distro, nil
		}
		return "", errors_package.New("Invalid distro: " + distro)
	}
	return api.hostDistroName, nil
}

// getDistro returns the named distro or nil
// It excludes unsupported distros by first checking the api.distros list
func (api *API) getDistro(name string) distro.Distro {
	if !common.IsStringInSortedSlice(api.distros, name) {
		return nil
	}
	return api.distroRegistry.GetDistro(name)
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

	repos, err := api.repoRegistry.ReposByArchName(api.hostDistroName, api.archName, false)
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
		data, err = ioutil.ReadAll(body)
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
		err = errors_package.New("blueprint must be in json or toml format")
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
		data, err = ioutil.ReadAll(body)
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
		err = errors_package.New("blueprint must be in json or toml format")
	}

	if err == nil && len(source.GetKey()) == 0 {
		err = errors_package.New("'id' field is missing from request")
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
			err = errors_package.New("'name' field is missing from request")
		} else if len(source.GetType()) == 0 {
			err = errors_package.New("'type' field is missing from request")
		} else if len(source.SourceConfig().URL) == 0 {
			err = errors_package.New("'url' field is missing from request")
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
		if !common.IsStringInSortedSlice(api.distros, d) {
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

	// Optional distro parameter
	// If it is empty it will return api.hostDistroName
	distroName, err := api.parseDistro(request.URL.Query())
	if err != nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
	availablePackages, err := api.fetchPackageList(distroName)

	if err != nil {
		errors := responseError{
			ID:  "ModulesError",
			Msg: fmt.Sprintf("msg: %s", err.Error()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	var packages rpmmd.PackageList
	if modulesParam != "" && modulesParam != "/" {
		// we have modules for search

		// remove leading /
		modulesParam = modulesParam[1:]

		names := strings.Split(modulesParam, ",")

		packages, err = availablePackages.Search(names...)

		if err != nil {
			errors := responseError{
				ID:  "ModulesError",
				Msg: fmt.Sprintf("Wrong glob pattern: %s", err.Error()),
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
	} else {
		// just return all available packages
		packages = availablePackages
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

	// Optional distro parameter
	// If it is empty it will return api.hostDistroName
	distroName, err := api.parseDistro(request.URL.Query())
	if err != nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
	availablePackages, err := api.fetchPackageList(distroName)

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

	// Optional distro parameter
	// If it is empty it will return api.hostDistroName
	distroName, err := api.parseDistro(request.URL.Query())
	if err != nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}
	availablePackages, err := api.fetchPackageList(distroName)

	if err != nil {
		errors := responseError{
			ID:  "ModulesError",
			Msg: fmt.Sprintf("msg: %s", err.Error()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	foundPackages, err := availablePackages.Search(names...)

	if err != nil {
		errors := responseError{
			ID:  errorId,
			Msg: fmt.Sprintf("Wrong glob pattern: %s", err.Error()),
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
		repos, err := api.allRepositories(distroName)
		if err != nil {
			errors := responseError{
				ID:  "InternalError",
				Msg: fmt.Sprintf("error while getting system repos: %v", err),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}
		d := api.getDistro(distroName)
		if d == nil {
			errors := responseError{
				ID:  "DistroError",
				Msg: fmt.Sprintf("Unknown distribution: %s", distroName),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}

		solver := api.solver.NewWithConfig(d.ModulePlatformID(), d.Releasever(), api.archName)
		for i := range packageInfos {
			pkgName := packageInfos[i].Name
			solved, err := solver.Depsolve([]rpmmd.PackageSet{{Include: []string{pkgName}, Repositories: repos}})
			if err != nil {
				errors := responseError{
					ID:  errorId,
					Msg: fmt.Sprintf("Cannot depsolve package %s: %s", packageInfos[i].Name, err.Error()),
				}
				statusResponseError(writer, http.StatusBadRequest, errors)
				return
			}
			packageInfos[i].Dependencies = solved
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

	// Optional distro parameter
	// If it is empty it will return api.hostDistroName
	distroName, err := api.parseDistro(request.URL.Query())
	if err != nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	d := api.getDistro(distroName)
	if d == nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: fmt.Sprintf("Unknown distribution: %s", distroName),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	repos, err := api.allRepositories(distroName)
	if err != nil {
		errors := responseError{
			ID:  "ProjectsError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	solver := api.solver.NewWithConfig(d.ModulePlatformID(), d.Releasever(), api.archName)
	deps, err := solver.Depsolve([]rpmmd.PackageSet{{Include: names, Repositories: repos}})
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
	err = json.NewEncoder(writer).Encode(reply{Projects: deps})
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

// setPkgEVRA replaces the version globs in the blueprint with their EVRA values from the dependencies
//
// The dependencies must be pre-sorted for this function to work properly
// It will return an error if it cannot find a package in the dependencies
func setPkgEVRA(dependencies []rpmmd.PackageSpec, packages []blueprint.Package) error {
	for pkgIndex, pkg := range packages {
		i := sort.Search(len(dependencies), func(i int) bool {
			return dependencies[i].Name >= pkg.Name
		})
		if i < len(dependencies) && dependencies[i].Name == pkg.Name {
			if dependencies[i].Epoch == 0 {
				packages[pkgIndex].Version = fmt.Sprintf("%s-%s.%s", dependencies[i].Version, dependencies[i].Release, dependencies[i].Arch)
			} else {
				packages[pkgIndex].Version = fmt.Sprintf("%d:%s-%s.%s", dependencies[i].Epoch, dependencies[i].Version, dependencies[i].Release, dependencies[i].Arch)
			}
		} else {
			// Packages should not be missing from the depsolve results
			return fmt.Errorf("%s missing from depsolve results", pkg.Name)
		}
	}
	return nil
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

		err = setPkgEVRA(dependencies, blueprint.Packages)
		if err != nil {
			rerr := responseError{
				ID:  "BlueprintsError",
				Msg: fmt.Sprintf("%s: %s", name, err.Error()),
			}
			errors = append(errors, rerr)
			break
		}

		err = setPkgEVRA(dependencies, blueprint.Modules)
		if err != nil {
			rerr := responseError{
				ID:  "BlueprintsError",
				Msg: fmt.Sprintf("%s: %s", name, err.Error()),
			}
			errors = append(errors, rerr)
			break
		}
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
		err = errors_package.New("blueprint must be in json or toml format")
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
		if !common.IsStringInSortedSlice(api.distros, blueprint.Distro) {
			errors := responseError{
				ID:  "BlueprintsError",
				Msg: fmt.Sprintf("'%s' is not a valid distribution", blueprint.Distro),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}
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
		err = errors_package.New("blueprint must be in json or toml format")
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

// depsolveBlueprintForImageType handles depsolving the blueprint package list and
// the packages required for the image type.
// NOTE: The imageType *must* be from the same distribution as the blueprint.
func (api *API) depsolveBlueprintForImageType(bp blueprint.Blueprint, options distro.ImageOptions, imageType distro.ImageType) (map[string][]rpmmd.PackageSpec, error) {
	// Depsolve using the host distro if none has been specified
	if bp.Distro == "" {
		bp.Distro = api.hostDistroName
	}

	if bp.Distro != imageType.Arch().Distro().Name() {
		return nil, fmt.Errorf("Blueprint distro %s does not match imageType distro %s", bp.Distro, imageType.Arch().Distro().Name())
	}

	imageTypeRepos, err := api.allRepositoriesByImageType(imageType)
	if err != nil {
		return nil, err
	}
	platformID := imageType.Arch().Distro().ModulePlatformID()
	releasever := imageType.Arch().Distro().Releasever()
	solver := api.solver.NewWithConfig(platformID, releasever, api.archName)

	packageSets := imageType.PackageSets(bp, options, imageTypeRepos)
	depsolvedSets := make(map[string][]rpmmd.PackageSpec, len(packageSets))

	for name, pkgSet := range packageSets {
		res, err := solver.Depsolve(pkgSet)
		if err != nil {
			return nil, err
		}
		depsolvedSets[name] = res
	}
	if err := solver.CleanCache(); err != nil {
		// log and ignore
		log.Printf("Error during rpm repo cache cleanup: %s", err.Error())
	}
	return depsolvedSets, nil
}

func (api *API) resolveContainersForImageType(bp blueprint.Blueprint, imageType distro.ImageType) ([]container.Spec, error) {
	resolver := container.NewResolver(imageType.Arch().Name())

	for _, c := range bp.Containers {
		resolver.Add(c.Source, c.Name, c.TLSVerify)
	}

	return resolver.Finish()
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
		OSTree        ostree.RequestParams `json:"ostree"`
		Branch        string               `json:"branch"`
		Upload        *uploadRequest       `json:"upload"`
	}
	type ComposeReply struct {
		BuildID uuid.UUID `json:"build_id"`
		Status  bool      `json:"status"`
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
	if api.getDistro(distroName) == nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: fmt.Sprintf("Unknown distribution: %s", distroName),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	// Get the imageType that corresponds to the distribution selected by the blueprint
	imageType, err := api.getImageType(distroName, cr.ComposeType)
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
	testMode := q.Get("test")

	if testMode == "1" || testMode == "2" {
		// Fake a parent commit for test requests
		cr.OSTree.Parent = "02604b2da6e954bd34b8b82a835e5a77d2b60ffa"
	} else {
		ostreeParams, err := ostree.ResolveParams(cr.OSTree, imageType.OSTreeRef())
		if err != nil {
			errors := responseError{
				ID:  "OSTreeOptionsError",
				Msg: err.Error(),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}
		cr.OSTree = ostreeParams
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

	options := distro.ImageOptions{
		Size: size,
		OSTree: ostree.RequestParams{
			Ref:    cr.OSTree.Ref,
			Parent: cr.OSTree.Parent,
			URL:    cr.OSTree.URL,
		},
	}

	packageSets, err := api.depsolveBlueprintForImageType(*bp, options, imageType)
	if err != nil {
		errors := responseError{
			ID:  "DepsolveError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}

	imageRepos, err := api.allRepositoriesByImageType(imageType)
	// this should not happen if the api.depsolveBlueprintForImageType() call above worked
	if err != nil {
		errors := responseError{
			ID:  "InternalError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}

	containerSpecs, err := api.resolveContainersForImageType(*bp, imageType)
	if err != nil {
		errors := responseError{
			ID:  "ContainerResolveError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}

	manifest, err := imageType.Manifest(bp.Customizations,
		options,
		imageRepos,
		packageSets,
		containerSpecs,
		seed)
	if err != nil {
		errors := responseError{
			ID:  "ManifestCreationFailed",
			Msg: fmt.Sprintf("failed to create osbuild manifest: %v", err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	var packages []rpmmd.PackageSpec
	// TODO: introduce a way to query these from the manifest / image type
	// BUG: installer/container image types will have empty package sets
	if packages = packageSets["packages"]; len(packages) == 0 {
		if packages = packageSets["os"]; len(packages) == 0 {
			packages = packageSets["ostree-tree"]
		}
	}

	if testMode == "1" {
		// Create a failed compose
		err = api.store.PushTestCompose(composeID, manifest, imageType, bp, size, targets, false, packages)
	} else if testMode == "2" {
		// Create a successful compose
		err = api.store.PushTestCompose(composeID, manifest, imageType, bp, size, targets, true, packages)
	} else {
		var jobId uuid.UUID

		jobId, err = api.workers.EnqueueOSBuild(api.archName, &worker.OSBuildJob{
			Manifest: manifest,
			Targets:  targets,
			PipelineNames: &worker.PipelineNames{
				Build:   imageType.BuildPipelines(),
				Payload: imageType.PayloadPipelines(),
			},
		}, "")
		if err == nil {
			err = api.store.PushCompose(composeID, manifest, imageType, bp, size, targets, jobId, packages)
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
		BuildID: composeID,
		Status:  true,
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

		composeStatus := api.getComposeStatus(compose)
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

	composeStatus := api.getComposeStatus(compose)
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
	// Optional distro parameter
	// If it is empty it will return api.hostDistroName
	distroName, err := api.parseDistro(request.URL.Query())
	if err != nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	d := api.getDistro(distroName)
	if d == nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: fmt.Sprintf("Unknown distribution: %s", distroName),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	// Get the distro specific arch so that we can return the correct list of image types
	arch, err := d.GetArch(api.archName)
	if err != nil {
		errors := responseError{
			ID:  "DistroError",
			Msg: fmt.Sprintf("Unknown arch: %s", api.archName),
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
		composeStatus := api.getComposeStatus(compose)
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
		composeStatus := api.getComposeStatus(compose)
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
			composeStatus := api.getComposeStatus(compose)
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
	composeStatus := api.getComposeStatus(compose)
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

	composeStatus := api.getComposeStatus(compose)
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

	composeStatus := api.getComposeStatus(compose)
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

	composeStatus := api.getComposeStatus(compose)
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

	composeStatus := api.getComposeStatus(compose)
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

	composeStatus := api.getComposeStatus(compose)
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
		composeStatus := api.getComposeStatus(compose)
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
		composeStatus := api.getComposeStatus(compose)
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
func (api *API) fetchPackageList(distroName string) (rpmmd.PackageList, error) {
	d := api.getDistro(distroName)
	if d == nil {
		return nil, fmt.Errorf("GetDistro - unknown distribution: %s", distroName)
	}
	repos, err := api.allRepositories(distroName)
	if err != nil {
		return nil, err
	}

	solver := api.solver.NewWithConfig(d.ModulePlatformID(), d.Releasever(), api.archName)
	packages, err := solver.FetchMetadata(repos)
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
// The first returned slice represents the Base repos (depending on the image
// type), while the second value represents the payload repos (defined by user).
// The difference from allRepositories() is that this method may return additional repositories,
// which are needed to build the specific image type. The allRepositories() can't do this, because
// it is used in places where image types are not considered.
func (api *API) allRepositoriesByImageType(imageType distro.ImageType) ([]rpmmd.RepoConfig, error) {
	repos, err := api.repoRegistry.ReposByImageType(imageType)
	if err != nil {
		return nil, err
	}

	payloadRepos := api.payloadRepositories(imageType.Arch().Distro().Name())
	// tag payload repositories with the payload package set names and add them to list of repos
	for _, pr := range payloadRepos {
		pr.ImageTypeTags = imageType.PayloadPackageSets()
		repos = append(repos, pr)
	}

	return repos, nil
}

// Returns all configured repositories (base + sources) as rpmmd.RepoConfig
func (api *API) allRepositories(distroName string) ([]rpmmd.RepoConfig, error) {
	repos, err := api.repoRegistry.ReposByArchName(distroName, api.archName, false)
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

	d := api.getDistro(bp.Distro)
	if d == nil {
		return nil, fmt.Errorf("GetDistro - unknown distribution: %s", bp.Distro)
	}
	repos, err := api.allRepositories(bp.Distro)
	if err != nil {
		return nil, err
	}

	solver := api.solver.NewWithConfig(d.ModulePlatformID(), d.Releasever(), api.archName)
	solved, err := solver.Depsolve([]rpmmd.PackageSet{{Include: bp.GetPackages(), Repositories: repos}})
	if err != nil {
		return nil, err
	}

	if err := solver.CleanCache(); err != nil {
		// log and ignore
		log.Printf("Error during rpm repo cache cleanup: %s", err.Error())
	}
	return solved, nil
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

	var reply struct {
		Distros []string `json:"distros"`
	}
	reply.Distros = api.distros

	err := json.NewEncoder(writer).Encode(reply)
	common.PanicOnError(err)
}
