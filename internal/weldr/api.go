package weldr

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"

	"github.com/osbuild/osbuild-composer/internal/distro"
	_ "github.com/osbuild/osbuild-composer/internal/distro/fedora30"
)

type API struct {
	store *store.Store

	rpmmd    rpmmd.RPMMD
	repo     rpmmd.RepoConfig
	packages rpmmd.PackageList

	logger *log.Logger
	router *httprouter.Router
}

func New(rpmmd rpmmd.RPMMD, repo rpmmd.RepoConfig, packages rpmmd.PackageList, logger *log.Logger, store *store.Store) *API {
	// This needs to be shared with the worker API so that they can communicate with each other
	// builds := make(chan queue.Build, 200)
	api := &API{
		store:    store,
		rpmmd:    rpmmd,
		repo:     repo,
		packages: packages,
		logger:   logger,
	}

	api.router = httprouter.New()
	api.router.RedirectTrailingSlash = false
	api.router.RedirectFixedPath = false
	api.router.MethodNotAllowed = http.HandlerFunc(methodNotAllowedHandler)
	api.router.NotFound = http.HandlerFunc(notFoundHandler)

	api.router.GET("/api/status", api.statusHandler)
	api.router.GET("/api/v0/projects/source/list", api.sourceListHandler)
	api.router.GET("/api/v0/projects/source/info/*sources", api.sourceInfoHandler)
	api.router.POST("/api/v0/projects/source/new", api.sourceNewHandler)
	api.router.DELETE("/api/v0/projects/source/delete/*source", api.sourceDeleteHandler)

	api.router.GET("/api/v0/modules/list", api.modulesListAllHandler)
	api.router.GET("/api/v0/modules/list/:modules", api.modulesListHandler)

	// these are the same, except that modules/info also includes dependencies
	api.router.GET("/api/v0/modules/info/*modules", api.modulesInfoHandler)
	api.router.GET("/api/v0/projects/info/*modules", api.modulesInfoHandler)

	api.router.GET("/api/v0/blueprints/list", api.blueprintsListHandler)
	api.router.GET("/api/v0/blueprints/info/*blueprints", api.blueprintsInfoHandler)
	api.router.GET("/api/v0/blueprints/depsolve/*blueprints", api.blueprintsDepsolveHandler)
	api.router.GET("/api/v0/blueprints/diff/:blueprint/:from/:to", api.blueprintsDiffHandler)
	api.router.GET("/api/v0/blueprints/changes/*blueprints", api.blueprintsChangesHandler)
	api.router.POST("/api/v0/blueprints/new", api.blueprintsNewHandler)
	api.router.POST("/api/v0/blueprints/workspace", api.blueprintsWorkspaceHandler)
	api.router.DELETE("/api/v0/blueprints/delete/:blueprint", api.blueprintDeleteHandler)
	api.router.DELETE("/api/v0/blueprints/workspace/:blueprint", api.blueprintDeleteWorkspaceHandler)

	api.router.POST("/api/v0/compose", api.composeHandler)
	api.router.GET("/api/v0/compose/types", api.composeTypesHandler)
	api.router.GET("/api/v0/compose/queue", api.composeQueueHandler)
	api.router.GET("/api/v0/compose/status/:uuids", api.composeStatusHandler)
	api.router.GET("/api/v0/compose/finished", api.composeFinishedHandler)
	api.router.GET("/api/v0/compose/failed", api.composeFailedHandler)
	api.router.GET("/api/v0/compose/image/:uuid", api.composeImageHandler)

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

func methodNotAllowedHandler(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusMethodNotAllowed)
}

func notFoundHandler(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusNotFound)
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

func (api *API) sourceListHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	type reply struct {
		Sources []string `json:"sources"`
	}

	names := api.store.ListSources()
	names = append(names, api.repo.Id)

	json.NewEncoder(writer).Encode(reply{
		Sources: names,
	})
}

func (api *API) sourceInfoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	// weldr uses a slightly different format than dnf to store repository
	// configuration
	type reply struct {
		Sources map[string]store.SourceConfig `json:"sources"`
		Errors  []responseError               `json:"errors"`
	}

	names := strings.Split(params.ByName("sources"), ",")

	if names[0] == "/" {
		errors := responseError{
			Code: http.StatusNotFound,
			ID:   "HTTPError",
			Msg:  "Not Found",
		}
		statusResponseError(writer, http.StatusNotFound, errors)
		return
	}

	sources := map[string]store.SourceConfig{}
	errors := []responseError{}
	for i, name := range names {
		// remove leading / from first name
		if i == 0 {
			name = name[1:]
		}
		// if name is "*" we want all sources from the store and the base repo
		if name == "*" {
			sources = api.store.GetAllSources()
		}
		// check if the source is in the base repo
		if name == api.repo.Id || name == "*" {
			cfg := store.SourceConfig{
				Name:     api.repo.Name,
				CheckGPG: true,
				CheckSSL: true,
				System:   true,
			}

			if api.repo.BaseURL != "" {
				cfg.URL = api.repo.BaseURL
				cfg.Type = "yum-baseurl"
			} else if api.repo.Metalink != "" {
				cfg.URL = api.repo.Metalink
				cfg.Type = "yum-metalink"
			} else if api.repo.MirrorList != "" {
				cfg.URL = api.repo.MirrorList
				cfg.Type = "yum-mirrorlist"
			}
			sources[cfg.Name] = cfg
			// check if the source is in the store
		} else if source := api.store.GetSource(name); source != nil {
			sources[source.Name] = *source
		} else {
			error := responseError{
				ID:  "UnknownSource",
				Msg: fmt.Sprintf("%s is not a valid source", name),
			}
			errors = append(errors, error)
		}
	}

	json.NewEncoder(writer).Encode(reply{
		Sources: sources,
		Errors:  errors,
	})
}

func (api *API) sourceNewHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
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

type modulesListModule struct {
	Name      string `json:"name"`
	GroupType string `json:"group_type"`
}

type modulesListReply struct {
	Total   uint                `json:"total"`
	Offset  uint                `json:"offset"`
	Limit   uint                `json:"limit"`
	Modules []modulesListModule `json:"modules"`
}

func (api *API) modulesListAllHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	offset, limit, err := parseOffsetAndLimit(request.URL.Query())
	if err != nil {
		errors := responseError{
			ID:  "BadLimitOrOffset",
			Msg: fmt.Sprintf("BadRequest: %s", err.Error()),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	total := uint(len(api.packages))
	start := min(offset, total)
	n := min(limit, total-start)

	modules := make([]modulesListModule, n)
	for i := uint(0); i < n; i++ {
		modules[i] = modulesListModule{api.packages[start+i].Name, "rpm"}
	}

	json.NewEncoder(writer).Encode(modulesListReply{
		Total:   total,
		Offset:  offset,
		Limit:   limit,
		Modules: modules,
	})
}

func (api *API) modulesListHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	names := strings.Split(params.ByName("modules"), ",")
	if names[0] == "" || names[0] == "*" {
		api.modulesListAllHandler(writer, request, params)
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

	// we don't support glob-matching, but cockpit-composer surrounds some
	// queries with asterisks; this is crude, but solves that case
	for i := range names {
		names[i] = strings.ReplaceAll(names[i], "*", "")
	}

	modules := make([]modulesListModule, 0)
	total := uint(0)
	end := offset + limit
	for i, name := range names {
		for _, pkg := range api.packages {
			if strings.Contains(pkg.Name, name) {
				total++
				if total > offset && total < end {
					modules = append(modules, modulesListModule{pkg.Name, "rpm"})
					// this removes names that have been found from the list of names
					if len(names) < i-1 {
						names = append(names[:i], names[i+1:]...)
					} else {
						names = names[:i]
					}
				}
			}
		}
	}

	// if a name remains in the list it was not found
	if len(names) > 0 {
		errors := responseError{
			ID:  "UnknownModule",
			Msg: fmt.Sprintf("one of the requested modules does not exist: ['%s']", names[0]),
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	json.NewEncoder(writer).Encode(modulesListReply{
		Total:   total,
		Offset:  offset,
		Limit:   limit,
		Modules: modules,
	})
}

func (api *API) modulesInfoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	type source struct {
		License string `json:"license"`
		Version string `json:"version"`
	}
	type build struct {
		Arch      string    `json:"arch"`
		BuildTime time.Time `json:"build_time"`
		Epoch     uint      `json:"epoch"`
		Release   string    `json:"release"`
		Source    source    `json:"source"`
	}
	type project struct {
		Name         string              `json:"name"`
		Summary      string              `json:"summary"`
		Description  string              `json:"description"`
		Homepage     string              `json:"homepage"`
		Builds       []build             `json:"builds"`
		Dependencies []rpmmd.PackageSpec `json:"dependencies,omitempty"`
	}
	type projectsReply struct {
		Projects []project `json:"projects"`
	}
	type modulesReply struct {
		Modules []project `json:"modules"`
	}

	// handle both projects/info and modules/info, the latter includes dependencies
	modulesRequested := strings.HasPrefix(request.URL.Path, "/api/v0/modules")

	names := strings.Split(params.ByName("modules"), ",")
	if names[0] == "/" {
		errors := responseError{
			Code: http.StatusNotFound,
			ID:   "HTTPError",
			Msg:  "Not Found",
		}
		statusResponseError(writer, http.StatusNotFound, errors)
		return
	}

	projects := make([]project, 0)
	for i, name := range names {
		// remove leading / from first name
		if i == 0 {
			name = name[1:]
		}

		first, n := api.packages.Search(name)
		if n == 0 {
			errors := responseError{
				ID:  "UnknownModule",
				Msg: fmt.Sprintf("one of the requested modules does not exist: %s", name),
			}
			statusResponseError(writer, http.StatusBadRequest, errors)
			return
		}

		// get basic info from the first package, but collect build
		// information from all that have the same name
		pkg := api.packages[first]
		project := project{
			Name:        pkg.Name,
			Summary:     pkg.Summary,
			Description: pkg.Description,
			Homepage:    pkg.URL,
		}

		project.Builds = make([]build, n)
		for i, pkg := range api.packages[first : first+n] {
			project.Builds[i] = build{
				Arch:      pkg.Arch,
				BuildTime: pkg.BuildTime,
				Epoch:     pkg.Epoch,
				Release:   pkg.Release,
				Source:    source{pkg.License, pkg.Version},
			}
		}

		if modulesRequested {
			project.Dependencies, _ = api.rpmmd.Depsolve([]string{pkg.Name}, []rpmmd.RepoConfig{api.repo})
		}

		projects = append(projects, project)
	}

	if modulesRequested {
		json.NewEncoder(writer).Encode(modulesReply{projects})
	} else {
		json.NewEncoder(writer).Encode(projectsReply{projects})
	}
}

func (api *API) blueprintsListHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
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

	json.NewEncoder(writer).Encode(reply{
		Blueprints: blueprints,
		Changes:    changes,
		Errors:     []responseError{},
	})
}

func (api *API) blueprintsDepsolveHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
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
		dependencies, _ := api.rpmmd.Depsolve(specs, []rpmmd.RepoConfig{api.repo})

		blueprints = append(blueprints, entry{blueprint, dependencies})
	}

	json.NewEncoder(writer).Encode(reply{
		Blueprints: blueprints,
		Errors:     []responseError{},
	})
}

func (api *API) blueprintsDiffHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
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

func (api *API) blueprintsNewHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
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

	api.store.PushBlueprint(blueprint)

	statusResponseOK(writer)
}

func (api *API) blueprintsWorkspaceHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
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

func (api *API) blueprintDeleteHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	api.store.DeleteBlueprint(params.ByName("blueprint"))
	statusResponseOK(writer)
}

func (api *API) blueprintDeleteWorkspaceHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	api.store.DeleteBlueprintFromWorkspace(params.ByName("blueprint"))
	statusResponseOK(writer)
}

// Schedule new compose by first translating the appropriate blueprint into a pipeline and then
// pushing it into the channel for waiting builds.
func (api *API) composeHandler(writer http.ResponseWriter, httpRequest *http.Request, _ httprouter.Params) {
	// https://weldr.io/lorax/pylorax.api.html#pylorax.api.v0.v0_compose_start
	type ComposeRequest struct {
		BlueprintName string `json:"blueprint_name"`
		ComposeType   string `json:"compose_type"`
		Branch        string `json:"branch"`
	}
	type ComposeReply struct {
		BuildID uuid.UUID `json:"build_id"`
		Status  bool      `json:"status"`
	}

	contentType := httpRequest.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		errors := responseError{
			ID:  "MissingPost",
			Msg: "blueprint must be json",
		}
		statusResponseError(writer, http.StatusBadRequest, errors)
		return
	}

	var cr ComposeRequest
	err := json.NewDecoder(httpRequest.Body).Decode(&cr)
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

	bp := blueprint.Blueprint{}
	changed := false
	found := api.store.GetBlueprint(cr.BlueprintName, &bp, &changed) // TODO: what to do with changed?

	if found {
		err := api.store.PushCompose(reply.BuildID, &bp, cr.ComposeType)

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

func (api *API) composeTypesHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	type composeType struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}

	var reply struct {
		Types []composeType `json:"types"`
	}

	d := distro.New("")
	for _, format := range d.ListOutputFormats() {
		reply.Types = append(reply.Types, composeType{format, true})
	}

	json.NewEncoder(writer).Encode(reply)
}

func (api *API) composeQueueHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	var reply struct {
		New []*store.ComposeEntry `json:"new"`
		Run []*store.ComposeEntry `json:"run"`
	}

	reply.New = make([]*store.ComposeEntry, 0)
	reply.Run = make([]*store.ComposeEntry, 0)

	for _, entry := range api.store.ListQueue(nil) {
		switch entry.QueueStatus {
		case "WAITING":
			reply.New = append(reply.New, entry)
		case "RUNNING":
			reply.Run = append(reply.Run, entry)
		}
	}

	json.NewEncoder(writer).Encode(reply)
}

func (api *API) composeStatusHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	var reply struct {
		UUIDs []*store.ComposeEntry `json:"uuids"`
	}

	uuidStrings := strings.Split(params.ByName("uuids"), ",")
	uuids := make([]uuid.UUID, len(uuidStrings))
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
	reply.UUIDs = api.store.ListQueue(uuids)

	json.NewEncoder(writer).Encode(reply)
}

func (api *API) composeImageHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
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

func (api *API) composeFinishedHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	var reply struct {
		Finished []interface{} `json:"finished"`
	}

	reply.Finished = make([]interface{}, 0)

	json.NewEncoder(writer).Encode(reply)
}

func (api *API) composeFailedHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	var reply struct {
		Failed []interface{} `json:"failed"`
	}

	reply.Failed = make([]interface{}, 0)

	json.NewEncoder(writer).Encode(reply)
}
