package weldr

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	errors_package "errors"
	"fmt"
	"io"
	"log"
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
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type API struct {
	store   *store.Store
	workers *worker.Server

	rpmmd  rpmmd.RPMMD
	arch   distro.Arch
	distro distro.Distro
	repos  []rpmmd.RepoConfig

	logger *log.Logger
	router *httprouter.Router

	compatOutputDir string
}

var ValidBlueprintName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func New(rpmmd rpmmd.RPMMD, arch distro.Arch, distro distro.Distro, repos []rpmmd.RepoConfig, logger *log.Logger, store *store.Store, workers *worker.Server, compatOutputDir string) *API {
	api := &API{
		store:           store,
		workers:         workers,
		rpmmd:           rpmmd,
		arch:            arch,
		distro:          distro,
		repos:           repos,
		logger:          logger,
		compatOutputDir: compatOutputDir,
	}

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
	api.router.GET("/api/v:version/compose/logs/:uuid", api.composeLogsHandler)
	api.router.GET("/api/v:version/compose/log/:uuid", api.composeLogHandler)
	api.router.POST("/api/v:version/compose/uploads/schedule/:uuid", api.uploadsScheduleHandler)

	api.router.DELETE("/api/v:version/upload/delete/:uuid", api.uploadsDeleteHandler)
	api.router.GET("/api/v:version/upload/info/:uuid", api.uploadsInfoHandler)
	api.router.GET("/api/v:version/upload/log/:uuid", api.uploadsLogHandler)
	api.router.POST("/api/v:version/upload/reset/:uuid", api.uploadsResetHandler)
	api.router.DELETE("/api/v:version/upload/cancel/:uuid", api.uploadsCancelHandler)

	api.router.GET("/api/v:version/upload/providers", api.providersHandler)
	api.router.POST("/api/v:version/upload/providers/save", api.providersSaveHandler)
	api.router.DELETE("/api/v:version/upload/providers/delete/:provider/:profile", api.providersDeleteHandler)

	return api
}

func (api *API) Serve(listener net.Listener) error {
	server := http.Server{Handler: api}

	err := server.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (api *API) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if api.logger != nil {
		log.Println(request.Method, request.URL.Path)
	}

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	api.router.ServeHTTP(writer, request)
}

type composeStatus struct {
	State    common.ComposeState
	Queued   time.Time
	Started  time.Time
	Finished time.Time
	Result   *common.ComposeResult
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
		var state common.ComposeState
		switch compose.ImageBuild.QueueStatus {
		case common.IBWaiting:
			state = common.CWaiting
		case common.IBRunning:
			state = common.CRunning
		case common.IBFinished:
			state = common.CFinished
		case common.IBFailed:
			state = common.CFailed
		}
		return &composeStatus{
			State:    state,
			Queued:   compose.ImageBuild.JobCreated,
			Started:  compose.ImageBuild.JobStarted,
			Finished: compose.ImageBuild.JobFinished,
			Result:   &common.ComposeResult{},
		}
	}

	// is it ok to ignore this error?
	jobStatus, _ := api.workers.JobStatus(jobId)
	return &composeStatus{
		State:    jobStatus.State,
		Queued:   jobStatus.Queued,
		Started:  jobStatus.Started,
		Finished: jobStatus.Finished,
		Result:   jobStatus.Result.OSBuildOutput,
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

// verifyStringsWithRegex checks a slive of strings against a regex of allowed characters
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
		Build:         "devel",
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

	names := api.store.ListSources()

	for _, repo := range api.repos {
		names = append(names, repo.Name)
	}

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

func (api *API) sourceInfoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) { // TODO: version 1 API
		return
	}

	// weldr uses a slightly different format than dnf to store repository
	// configuration
	type reply struct {
		Sources map[string]store.SourceConfig `json:"sources"`
		Errors  []responseError               `json:"errors"`
	}

	names := params.ByName("sources")

	sources := map[string]store.SourceConfig{}
	errors := []responseError{}

	// if names is "*" we want all sources
	if names == "*" {
		sources = api.store.GetAllSources()
		for _, repo := range api.repos {
			sources[repo.Name] = store.NewSourceConfig(repo, true)
		}
	} else {
		for _, name := range strings.Split(names, ",") {
			// check if the source is one of the base repos
			found := false
			for _, repo := range api.repos {
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
				sources[source.Name] = *source
			} else {
				error := responseError{
					ID:  "UnknownSource",
					Msg: fmt.Sprintf("%s is not a valid source", name),
				}
				errors = append(errors, error)
			}
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

	format := q.Get("format")
	if format == "json" || format == "" {
		err := json.NewEncoder(writer).Encode(reply{
			Sources: sources,
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
		return
	}
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

	var source SourceConfigV0
	var err error
	if contentType[0] == "application/json" {
		err = json.NewDecoder(request.Body).Decode(&source)
	} else if contentType[0] == "text/x-toml" {
		_, err = toml.DecodeReader(request.Body, &source)
	} else {
		err = errors_package.New("blueprint must be in json or toml format")
	}

	// Basic check of the source, should at least have a name and type
	if err == nil {
		if len(source.Name) == 0 {
			err = errors_package.New("'name' field is missing from API v0 request")
		} else if len(source.Type) == 0 {
			err = errors_package.New("'type' field is missing from API v0 request")
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

	api.store.PushSource(source.SourceConfig())

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

	// remove leading / from first name
	api.store.DeleteSource(name[0][1:])

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

	availablePackages, err := api.fetchPackageList()

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

	availablePackages, err := api.fetchPackageList()

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
	modulesRequested := strings.HasPrefix(request.URL.Path, "/api/v0/modules")

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

	availablePackages, err := api.fetchPackageList()

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
		for i := range packageInfos {
			err := packageInfos[i].FillDependencies(api.rpmmd, api.repos, api.distro.ModulePlatformID(), api.arch.Name())
			if err != nil {
				errors := responseError{
					ID:  errorId,
					Msg: fmt.Sprintf("Cannot depsolve package %s: %s", packageInfos[i].Name, err.Error()),
				}
				statusResponseError(writer, http.StatusBadRequest, errors)
				return
			}
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

	packages, _, err := api.rpmmd.Depsolve(names, nil, api.repos, api.distro.ModulePlatformID(), api.arch.Name())

	if err != nil {
		errors := responseError{
			ID:  "PROJECTS_ERROR",
			Msg: fmt.Sprintf("BadRequest: %s", err.Error()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	err = json.NewEncoder(writer).Encode(reply{
		Projects: packages,
	})
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

		dependencies, _, err := api.depsolveBlueprint(blueprint, nil)

		if err != nil {
			errors := responseError{
				ID:  "BlueprintsError",
				Msg: fmt.Sprintf("%s: %s", name, err.Error()),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
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
				Msg: fmt.Sprintf("%s: blueprint_not_found", name),
			}
			errors = append(errors, rerr)
			break
		}
		// Make a copy of the blueprint since we will be replacing the version globs
		blueprint := bp.DeepCopy()
		dependencies, _, err := api.depsolveBlueprint(&blueprint, nil)
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

	err := json.NewEncoder(writer).Encode(reply{
		Blueprints: blueprints,
		Errors:     errors,
	})
	common.PanicOnError(err)
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
		_, err = toml.DecodeReader(request.Body, &blueprint)
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
		_, err = toml.DecodeReader(request.Body, &blueprint)
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
			ID:  "BlueprintsError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	bp := bpChange.Blueprint
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

// Schedule new compose by first translating the appropriate blueprint into a pipeline and then
// pushing it into the channel for waiting builds.
func (api *API) composeHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type OSTreeRequest struct {
		Ref    string `json:"ref"`
		Parent string `json:"parent"`
	}

	// https://weldr.io/lorax/pylorax.api.html#pylorax.api.v0.v0_compose_start
	type ComposeRequest struct {
		BlueprintName string         `json:"blueprint_name"`
		ComposeType   string         `json:"compose_type"`
		Size          uint64         `json:"size"`
		OSTree        OSTreeRequest  `json:"ostree"`
		Branch        string         `json:"branch"`
		Upload        *uploadRequest `json:"upload"`
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

	imageType, err := api.arch.GetImageType(cr.ComposeType)
	if err != nil {
		errors := responseError{
			ID:  "UnknownComposeType",
			Msg: fmt.Sprintf("Unknown compose type for architecture: %s", cr.ComposeType),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	if !verifyStringsWithRegex(writer, []string{cr.BlueprintName}, ValidBlueprintName) {
		return
	}

	composeID := uuid.New()

	var targets []*target.Target
	if isRequestVersionAtLeast(params, 1) && cr.Upload != nil {
		t := uploadRequestToTarget(*cr.Upload, imageType)
		targets = append(targets, t)
	}

	targets = append(targets, target.NewLocalTarget(
		&target.LocalTargetOptions{
			ComposeId:    composeID,
			ImageBuildId: 0,
			Filename:     imageType.Filename(),
		},
	))

	bp := api.store.GetBlueprintCommitted(cr.BlueprintName)
	if bp == nil {
		errors := responseError{
			ID:  "UnknownBlueprint",
			Msg: fmt.Sprintf("Unknown blueprint name: %s", cr.BlueprintName),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	packages, buildPackages, err := api.depsolveBlueprint(bp, imageType)
	if err != nil {
		errors := responseError{
			ID:  "DepsolveError",
			Msg: err.Error(),
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
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

	size := imageType.Size(cr.Size)
	manifest, err := imageType.Manifest(bp.Customizations,
		distro.ImageOptions{
			Size: size,
			OSTree: distro.OSTreeImageOptions{
				Ref:    cr.OSTree.Ref,
				Parent: cr.OSTree.Parent,
			},
		},
		api.allRepositories(),
		packages,
		buildPackages)
	if err != nil {
		errors := responseError{
			ID:  "ManifestCreationFailed",
			Msg: fmt.Sprintf("failed to create osbuild manifest: %v", err),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	testMode := q.Get("test")
	if testMode == "1" {
		// Create a failed compose
		err = api.store.PushTestCompose(composeID, manifest, imageType, bp, size, targets, false)
	} else if testMode == "2" {
		// Create a successful compose
		err = api.store.PushTestCompose(composeID, manifest, imageType, bp, size, targets, true)
	} else {
		var jobId uuid.UUID

		jobId, err = api.workers.Enqueue(manifest, targets)
		if err == nil {
			err = api.store.PushCompose(composeID, manifest, imageType, bp, size, targets, jobId)
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
		if composeStatus.State != common.CFinished && composeStatus.State != common.CFailed {
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

func (api *API) composeTypesHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}
	type composeType struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}

	var reply struct {
		Types []composeType `json:"types"`
	}

	for _, format := range api.arch.ListImageTypes() {
		reply.Types = append(reply.Types, composeType{format, true})
	}

	err := json.NewEncoder(writer).Encode(reply)
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
		case common.CWaiting:
			reply.New = append(reply.New, composeToComposeEntry(id, compose, composeStatus, includeUploads))
		case common.CRunning:
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

	type Dependencies struct {
		Packages []map[string]interface{} `json:"packages"`
	}

	var reply struct {
		ID          uuid.UUID            `json:"id"`
		Config      string               `json:"config"`    // anaconda config, let's ignore this field
		Blueprint   *blueprint.Blueprint `json:"blueprint"` // blueprint not frozen!
		Commit      string               `json:"commit"`    // empty for now
		Deps        Dependencies         `json:"deps"`      // empty for now
		ComposeType string               `json:"compose_type"`
		QueueStatus string               `json:"queue_status"`
		ImageSize   uint64               `json:"image_size"`
		Uploads     []uploadResponse     `json:"uploads,omitempty"`
	}

	reply.ID = id
	reply.Blueprint = compose.Blueprint
	reply.Deps = Dependencies{
		Packages: make([]map[string]interface{}, 0),
	}
	// Weldr API assumes only one image build per compose, that's why only the
	// 1st build is considered
	composeStatus := api.getComposeStatus(compose)
	reply.ComposeType = compose.ImageBuild.ImageType.Name()
	reply.QueueStatus = composeStatus.State.ToString()
	reply.ImageSize = compose.ImageBuild.Size

	if isRequestVersionAtLeast(params, 1) {
		reply.Uploads = targetsToUploadResponses(compose.ImageBuild.Targets)
	}

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
	if composeStatus.State != common.CFinished {
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
	if composeStatus.State != common.CFinished && composeStatus.State != common.CFailed {
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
		Name: "logs/osbuild.log",
		Mode: 0644,
		Size: int64(fileContents.Len()),
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
	if composeStatus.State == common.CWaiting {
		errors := responseError{
			ID:  "BuildInWrongState",
			Msg: fmt.Sprintf("Build %s has not started yet. No logs to view.", uuidString),
		}
		statusResponseError(writer, http.StatusOK, errors) // weirdly, Lorax returns 200 in this case
		return
	}

	if composeStatus.State == common.CRunning {
		fmt.Fprintf(writer, "Running...\n")
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
		if composeStatus.State != common.CFinished {
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
		if composeStatus.State != common.CFailed {
			continue
		}
		reply.Failed = append(reply.Failed, composeToComposeEntry(id, compose, composeStatus, includeUploads))
	}
	sortComposeEntries(reply.Failed)

	err := json.NewEncoder(writer).Encode(reply)
	common.PanicOnError(err)
}

func (api *API) fetchPackageList() (rpmmd.PackageList, error) {
	packages, _, err := api.rpmmd.FetchMetadata(api.allRepositories(), api.distro.ModulePlatformID(), api.arch.Name())
	return packages, err
}

func getPkgNameGlob(pkg blueprint.Package) string {
	// If a package has version "*" the package name suffix must be equal to "-*-*.*"
	// Using just "-*" would find any other package containing the package name
	if pkg.Version == "*" {
		return fmt.Sprintf("%s-*-*.*", pkg.Name)
	} else if pkg.Version != "" {
		return fmt.Sprintf("%s-%s", pkg.Name, pkg.Version)
	}
	return pkg.Name
}

// Returns all configured repositories (base + sources) as rpmmd.RepoConfig
func (api *API) allRepositories() []rpmmd.RepoConfig {
	repos := append([]rpmmd.RepoConfig{}, api.repos...)
	for _, source := range api.store.GetAllSources() {
		repos = append(repos, source.RepoConfig())
	}
	return repos
}

func (api *API) depsolveBlueprint(bp *blueprint.Blueprint, imageType distro.ImageType) ([]rpmmd.PackageSpec, []rpmmd.PackageSpec, error) {
	repos := api.allRepositories()
	var specs []string = []string{}
	for _, pkg := range bp.Packages {
		specs = append(specs, getPkgNameGlob(pkg))
	}
	for _, mod := range bp.Modules {
		specs = append(specs, getPkgNameGlob(mod))
	}
	excludeSpecs := []string{}
	if imageType != nil {
		// When the output type is known, include the base packages in the depsolve
		// transaction.
		packages, excludePackages := imageType.BasePackages()
		specs = append(specs, packages...)
		excludeSpecs = append(excludePackages, excludeSpecs...)
	}

	packages, _, err := api.rpmmd.Depsolve(specs, excludeSpecs, repos, api.distro.ModulePlatformID(), api.arch.Name())
	if err != nil {
		return nil, nil, err
	}

	buildPackages := []rpmmd.PackageSpec{}
	if imageType != nil {
		buildSpecs := imageType.BuildPackages()
		buildPackages, _, err = api.rpmmd.Depsolve(buildSpecs, nil, repos, api.distro.ModulePlatformID(), api.arch.Name())
		if err != nil {
			return nil, nil, err
		}
	}

	return packages, buildPackages, err
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
