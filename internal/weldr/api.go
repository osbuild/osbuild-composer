package weldr

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/target"
)

type API struct {
	store *store.Store

	rpmmd  rpmmd.RPMMD
	distro distro.Distro

	logger *log.Logger
	router *httprouter.Router
}

func New(rpmmd rpmmd.RPMMD, distro distro.Distro, logger *log.Logger, store *store.Store) *API {
	// This needs to be shared with the worker API so that they can communicate with each other
	// builds := make(chan queue.Build, 200)
	api := &API{
		store:  store,
		rpmmd:  rpmmd,
		distro: distro,
		logger: logger,
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

	api.router.GET("/api/v:version/projects/depsolve/:projects", api.projectsDepsolveHandler)

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
	api.router.DELETE("/api/v:version/blueprints/delete/:blueprint", api.blueprintDeleteHandler)
	api.router.DELETE("/api/v:version/blueprints/workspace/:blueprint", api.blueprintDeleteWorkspaceHandler)

	api.router.POST("/api/v:version/compose", api.composeHandler)
	api.router.GET("/api/v:version/compose/types", api.composeTypesHandler)
	api.router.GET("/api/v:version/compose/queue", api.composeQueueHandler)
	api.router.GET("/api/v:version/compose/status/:uuids", api.composeStatusHandler)
	api.router.GET("/api/v:version/compose/info/:uuid", api.composeInfoHandler)
	api.router.GET("/api/v:version/compose/finished", api.composeFinishedHandler)
	api.router.GET("/api/v:version/compose/failed", api.composeFailedHandler)
	api.router.GET("/api/v:version/compose/image/:uuid", api.composeImageHandler)
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

	if err != nil {
		panic(err)
	}

	return uint(version) >= minVersion
}

func methodNotAllowedHandler(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusMethodNotAllowed)
}

func notFoundHandler(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusNotFound)
}

func notImplementedHandler(writer http.ResponseWriter, httpRequest *http.Request, _ httprouter.Params) {
	writer.WriteHeader(http.StatusNotImplemented)
}

func statusResponseOK(writer http.ResponseWriter) {
	type reply struct {
		Status bool `json:"status"`
	}

	writer.WriteHeader(http.StatusOK)
	json.NewEncoder(writer).Encode(reply{true})
}

type responseError struct {
	Code int    `json:"code,omitempty"`
	ID   string `json:"id"`
	Msg  string `json:"msg"`
}

func statusResponseError(writer http.ResponseWriter, code int, errors ...responseError) {
	type reply struct {
		Status bool            `json:"status"`
		Errors []responseError `json:"errors,omitempty"`
	}

	writer.WriteHeader(code)
	json.NewEncoder(writer).Encode(reply{false, errors})
}

func (api *API) statusHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	type reply struct {
		API           uint     `json:"api"`
		DBSupported   bool     `json:"db_supported"`
		DBVersion     string   `json:"db_version"`
		SchemaVersion string   `json:"schema_version"`
		Backend       string   `json:"backend"`
		Build         string   `json:"build"`
		Messages      []string `json:"messages"`
	}

	json.NewEncoder(writer).Encode(reply{
		API:           1,
		DBSupported:   true,
		DBVersion:     "0",
		SchemaVersion: "0",
		Backend:       "osbuild-composer",
		Build:         "devel",
		Messages:      make([]string, 0),
	})
}

func (api *API) sourceListHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type reply struct {
		Sources []string `json:"sources"`
	}

	names := api.store.ListSources()

	for _, repo := range api.distro.Repositories() {
		names = append(names, repo.Id)
	}

	json.NewEncoder(writer).Encode(reply{
		Sources: names,
	})
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
		for _, repo := range api.distro.Repositories() {
			sources[repo.Id] = store.NewSourceConfig(repo, true)
		}
	} else {
		for _, name := range strings.Split(names, ",") {
			// check if the source is one of the base repos
			found := false
			for _, repo := range api.distro.Repositories() {
				if name == repo.Id {
					sources[repo.Id] = store.NewSourceConfig(repo, true)
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

	json.NewEncoder(writer).Encode(reply{
		Sources: sources,
		Errors:  errors,
	})
}

func (api *API) sourceNewHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) { // TODO: version 1 API
		return
	}

	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		errors := responseError{
			ID:  "HTTPError",
			Msg: "Internal Server Error",
		}
		statusResponseError(writer, http.StatusInternalServerError, errors)
		return
	}

	var source store.SourceConfig
	err := json.NewDecoder(request.Body).Decode(&source)
	if err != nil {
		errors := responseError{
			Code: http.StatusBadRequest,
			ID:   "HTTPError",
			Msg:  "Bad Request",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	api.store.PushSource(source)

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

	json.NewEncoder(writer).Encode(reply{
		Total:   total,
		Offset:  offset,
		Limit:   limit,
		Modules: modules,
	})
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

	json.NewEncoder(writer).Encode(reply{
		Total:    total,
		Offset:   offset,
		Limit:    limit,
		Projects: packages,
	})
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
		for i, _ := range packageInfos {
			err := packageInfos[i].FillDependencies(api.rpmmd, api.distro.Repositories())
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
		json.NewEncoder(writer).Encode(modulesReply{packageInfos})
	} else {
		json.NewEncoder(writer).Encode(projectsReply{packageInfos})
	}
}

func (api *API) projectsDepsolveHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	type reply struct {
		Projects []rpmmd.PackageSpec `json:"projects"`
	}

	names := strings.Split(params.ByName("projects"), ",")

	packages, err := api.rpmmd.Depsolve(names, api.distro.Repositories())

	if err != nil {
		errors := responseError{
			ID:  "PROJECTS_ERROR",
			Msg: fmt.Sprintf("BadRequest: %s", err.Error()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	json.NewEncoder(writer).Encode(reply{
		Projects: packages,
	})
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

	json.NewEncoder(writer).Encode(reply{
		Total:      total,
		Offset:     offset,
		Limit:      limit,
		Blueprints: names[offset : offset+limit],
	})
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

	blueprints := []blueprint.Blueprint{}
	changes := []change{}

	for i, name := range names {
		// remove leading / from first name
		if i == 0 {
			name = name[1:]
		}
		var blueprint blueprint.Blueprint
		var changed bool
		if !api.store.GetBlueprint(name, &blueprint, &changed) {
			errors := responseError{
				ID:  "UnknownBlueprint",
				Msg: fmt.Sprintf("%s: ", name),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}
		blueprints = append(blueprints, blueprint)
		changes = append(changes, change{changed, blueprint.Name})
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
		json.NewEncoder(writer).Encode(reply{
			Blueprints: blueprints,
			Changes:    changes,
			Errors:     []responseError{},
		})
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
		toml.NewEncoder(writer).Encode(blueprints[0])
	} else {
		errors := responseError{
			ID:  "InvalidChars",
			Msg: fmt.Sprintf("invalid `format` parameter: %s", format),
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

	blueprints := []entry{}
	for i, name := range names {
		// remove leading / from first name
		if i == 0 {
			name = name[1:]
		}
		var blueprint blueprint.Blueprint
		if !api.store.GetBlueprint(name, &blueprint, nil) {
			errors := responseError{
				ID:  "UnknownBlueprint",
				Msg: fmt.Sprintf("%s: blueprint not found", name),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}

		specs := make([]string, len(blueprint.Packages))
		for i, pkg := range blueprint.Packages {
			specs[i] = pkg.Name
			// If a package has version "*" the package name suffix must be equal to "-*-*.*"
			// Using just "-*" would find any other package containing the package name
			if pkg.Version != "" && pkg.Version != "*" {
				specs[i] += "-" + pkg.Version
			} else if pkg.Version == "*" {
				specs[i] += "-*-*.*"
			}
		}

		var repos []rpmmd.RepoConfig
		for _, repo := range api.distro.Repositories() {
			repos = append(repos, repo)
		}
		for _, source := range api.store.GetAllSources() {
			repos = append(repos, source.RepoConfig())
		}

		dependencies, err := api.rpmmd.Depsolve(specs, repos)

		if err != nil {
			errors := responseError{
				ID:  "BlueprintsError",
				Msg: fmt.Sprintf("%s: %s", name, err.Error()),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}

		blueprints = append(blueprints, entry{blueprint, dependencies})
	}

	json.NewEncoder(writer).Encode(reply{
		Blueprints: blueprints,
		Errors:     []responseError{},
	})
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

	blueprints := []blueprintFrozen{}
	errors := []responseError{}
	for i, name := range names {
		// remove leading / from first name
		if i == 0 {
			name = name[1:]
		}
		var blueprint blueprint.Blueprint
		var changed bool
		if !api.store.GetBlueprint(name, &blueprint, &changed) {
			err := responseError{
				ID:  "UnknownBlueprint",
				Msg: fmt.Sprintf("%s: blueprint_not_found", name),
			}
			errors = append(errors, err)
			return
		}

		specs := make([]string, len(blueprint.Packages))
		for i, pkg := range blueprint.Packages {
			specs[i] = pkg.Name
			// If a package has version "*" the package name suffix must be equal to "-*-*.*"
			// Using just "-*" would find any other package containing the package name
			if pkg.Version != "" && pkg.Version != "*" {
				specs[i] += "-" + pkg.Version
			} else if pkg.Version == "*" {
				specs[i] += "-*-*.*"
			}
		}

		var repos []rpmmd.RepoConfig
		for _, repo := range api.distro.Repositories() {
			repos = append(repos, repo)
		}
		for _, source := range api.store.GetAllSources() {
			repos = append(repos, source.RepoConfig())
		}

		dependencies, _ := api.rpmmd.Depsolve(specs, repos)

		for pkgIndex, pkg := range blueprint.Packages {
			i := sort.Search(len(dependencies), func(i int) bool {
				return dependencies[i].Name >= pkg.Name
			})
			if i < len(dependencies) && dependencies[i].Name == pkg.Name {
				blueprint.Packages[pkgIndex].Version = dependencies[i].Version + "-" + dependencies[i].Release + "." + dependencies[i].Arch
			}
		}

		blueprints = append(blueprints, blueprintFrozen{blueprint})
	}

	json.NewEncoder(writer).Encode(reply{
		Blueprints: blueprints,
		Errors:     errors,
	})
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
	fromCommit := params.ByName("from")
	toCommit := params.ByName("to")

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
	var oldBlueprint, newBlueprint blueprint.Blueprint
	if !api.store.GetBlueprintCommitted(name, &oldBlueprint) || !api.store.GetBlueprint(name, &newBlueprint, nil) {
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

	json.NewEncoder(writer).Encode(reply{diffs})
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
	for i, name := range names {
		// remove leading / from first name
		if i == 0 {
			name = name[1:]
		}
		bpChanges := api.store.GetBlueprintChanges(name)
		if bpChanges != nil {
			change := change{
				Changes: bpChanges,
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

	json.NewEncoder(writer).Encode(reply{
		BlueprintsChanges: allChanges,
		Errors:            errors,
		Offset:            offset,
		Limit:             limit,
	})
}

func (api *API) blueprintsNewHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: "missing Content-Type header",
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

	commitMsg := "Recipe " + blueprint.Name + ", version " + blueprint.Version + " saved."
	api.store.PushBlueprint(blueprint, commitMsg)

	statusResponseOK(writer)
}

func (api *API) blueprintsWorkspaceHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: "'blueprint must be json'",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	var blueprint blueprint.Blueprint
	err := json.NewDecoder(request.Body).Decode(&blueprint)
	if err != nil {
		errors := responseError{
			ID:  "BlueprintsError",
			Msg: "400 Bad Request: The browser (or proxy) sent a request that this server could not understand.",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	api.store.PushBlueprintToWorkspace(blueprint)

	statusResponseOK(writer)
}

func (api *API) blueprintUndoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	name := params.ByName("blueprint")
	commit := params.ByName("commit")
	bpChange := api.store.GetBlueprintChange(name, commit)
	bp := bpChange.Blueprint
	commitMsg := name + ".toml reverted to commit " + commit
	api.store.PushBlueprint(bp, commitMsg)
	statusResponseOK(writer)
}

func (api *API) blueprintDeleteHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	api.store.DeleteBlueprint(params.ByName("blueprint"))
	statusResponseOK(writer)
}

func (api *API) blueprintDeleteWorkspaceHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	api.store.DeleteBlueprintFromWorkspace(params.ByName("blueprint"))
	statusResponseOK(writer)
}

// Schedule new compose by first translating the appropriate blueprint into a pipeline and then
// pushing it into the channel for waiting builds.
func (api *API) composeHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	// https://weldr.io/lorax/pylorax.api.html#pylorax.api.v0.v0_compose_start
	type ComposeRequest struct {
		BlueprintName string         `json:"blueprint_name"`
		ComposeType   string         `json:"compose_type"`
		Branch        string         `json:"branch"`
		Upload        *UploadRequest `json:"upload"`
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

	reply := ComposeReply{
		BuildID: uuid.New(),
		Status:  true,
	}

	var uploadTarget *target.Target
	if isRequestVersionAtLeast(params, 1) && cr.Upload != nil {
		uploadTarget, err = UploadRequestToTarget(*cr.Upload)

		if err != nil {
			errors := responseError{
				ID:  "UploadError",
				Msg: fmt.Sprintf("bad input format: %s", err.Error()),
			}

			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}
	}

	bp := blueprint.Blueprint{}
	changed := false
	found := api.store.GetBlueprint(cr.BlueprintName, &bp, &changed) // TODO: what to do with changed?

	if found {
		err := api.store.PushCompose(reply.BuildID, &bp, cr.ComposeType, uploadTarget)

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
	} else {
		errors := responseError{
			ID:  "UnknownBlueprint",
			Msg: fmt.Sprintf("Unknown blueprint name: %s", cr.BlueprintName),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	json.NewEncoder(writer).Encode(reply)
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

	for _, format := range api.distro.ListOutputFormats() {
		reply.Types = append(reply.Types, composeType{format, true})
	}

	json.NewEncoder(writer).Encode(reply)
}

func (api *API) composeQueueHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	reply := struct {
		New []*ComposeEntry `json:"new"`
		Run []*ComposeEntry `json:"run"`
	}{[]*ComposeEntry{}, []*ComposeEntry{}}

	composes := api.store.GetAllComposes()
	for id, compose := range composes {
		switch compose.QueueStatus {
		case "WAITING":
			reply.New = append(reply.New, composeToComposeEntry(id, compose, isRequestVersionAtLeast(params, 1)))
		case "RUNNING":
			reply.Run = append(reply.Run, composeToComposeEntry(id, compose, isRequestVersionAtLeast(params, 1)))
		}
	}

	json.NewEncoder(writer).Encode(reply)
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

	var uuids []uuid.UUID

	if uuidsParam != "*" {
		uuidStrings := strings.Split(uuidsParam, ",")
		uuids = make([]uuid.UUID, len(uuidStrings))
		for _, uuidString := range uuidStrings {
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
	}
	composes := api.store.GetAllComposes()

	reply.UUIDs = composesToComposeEntries(composes, uuids, isRequestVersionAtLeast(params, 1))

	json.NewEncoder(writer).Encode(reply)
}

func (api *API) composeInfoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	// TODO: implement this route for v0 and v1 (they differ)
	notImplementedHandler(writer, request, params)
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

	image, err := api.store.GetImage(uuid)
	if err != nil {
		errors := responseError{
			ID:  "BuildMissingFile",
			Msg: fmt.Sprintf("Build %s is missing image file %s", uuidString, image.Name),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	writer.Header().Set("Content-Disposition", "attachment; filename="+uuid.String()+"-"+image.Name)
	writer.Header().Set("Content-Type", image.Mime)
	writer.Header().Set("Content-Length", fmt.Sprintf("%d", image.Size))

	io.Copy(writer, image.File)
}

func (api *API) composeFinishedHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	reply := struct {
		Finished []*ComposeEntry `json:"finished"`
	}{[]*ComposeEntry{}}

	composes := api.store.GetAllComposes()
	for _, entry := range composesToComposeEntries(composes, nil, isRequestVersionAtLeast(params, 1)) {
		switch entry.QueueStatus {
		case "FINISHED":
			reply.Finished = append(reply.Finished, entry)
		}
	}

	json.NewEncoder(writer).Encode(reply)
}

func (api *API) composeFailedHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	if !verifyRequestVersion(writer, params, 0) {
		return
	}

	reply := struct {
		Failed []*ComposeEntry `json:"failed"`
	}{[]*ComposeEntry{}}

	composes := api.store.GetAllComposes()
	for _, entry := range composesToComposeEntries(composes, nil, isRequestVersionAtLeast(params, 1)) {
		switch entry.QueueStatus {
		case "FAILED":
			reply.Failed = append(reply.Failed, entry)
		}
	}

	json.NewEncoder(writer).Encode(reply)
}

func (api *API) fetchPackageList() (rpmmd.PackageList, error) {
	var repos []rpmmd.RepoConfig
	for _, repo := range api.distro.Repositories() {
		repos = append(repos, repo)
	}
	for _, source := range api.store.GetAllSources() {
		repos = append(repos, source.RepoConfig())
	}

	return api.rpmmd.FetchPackageList(repos)
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
