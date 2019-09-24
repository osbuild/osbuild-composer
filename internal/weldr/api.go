package weldr

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"osbuild-composer/internal/rpmmd"
)

type API struct {
	store *store

	repo     rpmmd.RepoConfig
	packages rpmmd.PackageList

	logger *log.Logger
	router *httprouter.Router
}

func New(repo rpmmd.RepoConfig, packages rpmmd.PackageList, logger *log.Logger, initialState []byte, stateChannel chan<- []byte) *API {
	api := &API{
		store:    newStore(initialState, stateChannel),
		repo:     repo,
		packages: packages,
		logger:   logger,
	}

	// sample blueprint on first run
	if initialState == nil {
		api.store.pushBlueprint(blueprint{
			Name:        "example",
			Description: "An Example",
			Version:     "1",
			Packages:    []blueprintPackage{{"httpd", "2.*"}},
			Modules:     []blueprintPackage{},
		})
	}

	api.router = httprouter.New()
	api.router.RedirectTrailingSlash = false
	api.router.RedirectFixedPath = false
	api.router.MethodNotAllowed = http.HandlerFunc(methodNotAllowedHandler)
	api.router.NotFound = http.HandlerFunc(notFoundHandler)

	api.router.GET("/api/status", api.statusHandler)
	api.router.GET("/api/v0/projects/source/list", api.sourceListHandler)
	api.router.GET("/api/v0/projects/source/info/:sources", api.sourceInfoHandler)

	api.router.GET("/api/v0/modules/list", api.modulesListAllHandler)
	api.router.GET("/api/v0/modules/list/:modules", api.modulesListHandler)

	// these are the same, except that modules/info also includes dependencies
	api.router.GET("/api/v0/modules/info/:modules", api.modulesInfoHandler)
	api.router.GET("/api/v0/projects/info/:modules", api.modulesInfoHandler)

	api.router.GET("/api/v0/blueprints/list", api.blueprintsListHandler)
	api.router.GET("/api/v0/blueprints/info/:blueprints", api.blueprintsInfoHandler)
	api.router.GET("/api/v0/blueprints/depsolve/:blueprints", api.blueprintsDepsolveHandler)
	api.router.GET("/api/v0/blueprints/diff/:blueprint/:from/:to", api.blueprintsDiffHandler)
	api.router.POST("/api/v0/blueprints/new", api.blueprintsNewHandler)
	api.router.POST("/api/v0/blueprints/workspace", api.blueprintsWorkspaceHandler)
	api.router.DELETE("/api/v0/blueprints/delete/:blueprint", api.blueprintDeleteHandler)
	api.router.DELETE("/api/v0/blueprints/workspace/:blueprint", api.blueprintDeleteWorkspaceHandler)

	api.router.GET("/api/v0/compose/queue", api.composeQueueHandler)
	api.router.GET("/api/v0/compose/finished", api.composeFinishedHandler)
	api.router.GET("/api/v0/compose/failed", api.composeFailedHandler)

	return api
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

func statusResponseError(writer http.ResponseWriter, code int, errors ...string) {
	type reply struct {
		Status bool     `json:"status"`
		Errors []string `json:"errors,omitempty"`
	}

	writer.WriteHeader(code)
	json.NewEncoder(writer).Encode(reply{false, errors})
}

func (api *API) statusHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	type reply struct {
		Api           uint     `json:"api"`
		DBSupported   bool     `json:"db_supported"`
		DBVersion     string   `json:"db_version"`
		SchemaVersion string   `json:"schema_version"`
		Backend       string   `json:"backend"`
		Build         string   `json:"build"`
		Messages      []string `json:"messages"`
	}

	json.NewEncoder(writer).Encode(reply{
		Api:           1,
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

	json.NewEncoder(writer).Encode(reply{
		Sources: []string{api.repo.Id},
	})
}

func (api *API) sourceInfoHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	// weldr uses a slightly different format than dnf to store repository
	// configuration
	type sourceConfig struct {
		Id       string `json:"id"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		URL      string `json:"url"`
		CheckGPG bool   `json:"check_gpg"`
		CheckSSL bool   `json:"check_ssl"`
		System   bool   `json:"system"`
	}
	type reply struct {
		Sources map[string]sourceConfig `json:"sources"`
	}

	// we only have one repository
	names := strings.Split(params.ByName("sources"), ",")
	if names[0] != api.repo.Id && names[0] != "*" {
		statusResponseError(writer, http.StatusBadRequest, "repository not found: "+names[0])
		return
	}

	cfg := sourceConfig{
		Id:       api.repo.Id,
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

	json.NewEncoder(writer).Encode(reply{
		Sources: map[string]sourceConfig{cfg.Id: cfg},
	})
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
		statusResponseError(writer, http.StatusBadRequest, "BadRequest: "+err.Error())
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
		statusResponseError(writer, http.StatusBadRequest, "BadRequest: "+err.Error())
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

	for _, pkg := range api.packages {
		for _, name := range names {
			if strings.Contains(pkg.Name, name) {
				total += 1
				if total > offset && total < end {
					modules = append(modules, modulesListModule{pkg.Name, "rpm"})
				}
			}
		}
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
	if names[0] == "" {
		statusResponseError(writer, http.StatusNotFound)
		return
	}

	projects := make([]project, 0)
	for _, name := range names {
		first, n := api.packages.Search(name)
		if n == 0 {
			statusResponseError(writer, http.StatusNotFound)
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
			project.Dependencies, _ = rpmmd.Depsolve(pkg.Name)
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
		statusResponseError(writer, http.StatusBadRequest, "BadRequest: "+err.Error())
		return
	}

	names := api.store.listBlueprints()
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
		Blueprints []blueprint `json:"blueprints"`
		Changes    []change    `json:"changes"`
		Errors     []string    `json:"errors"`
	}

	names := strings.Split(params.ByName("blueprints"), ",")
	if names[0] == "" {
		statusResponseError(writer, http.StatusNotFound)
		return
	}

	blueprints := []blueprint{}
	changes := []change{}
	for _, name := range names {
		var blueprint blueprint
		var changed bool
		if !api.store.getBlueprint(name, &blueprint, &changed) {
			statusResponseError(writer, http.StatusNotFound)
			return
		}
		blueprints = append(blueprints, blueprint)
		changes = append(changes, change{changed, blueprint.Name})
	}

	json.NewEncoder(writer).Encode(reply{
		Blueprints: blueprints,
		Changes:    changes,
		Errors:     []string{},
	})
}

func (api *API) blueprintsDepsolveHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	type entry struct {
		Blueprint    blueprint           `json:"blueprint"`
		Dependencies []rpmmd.PackageSpec `json:"dependencies"`
	}
	type reply struct {
		Blueprints []entry  `json:"blueprints"`
		Errors     []string `json:"errors"`
	}

	names := strings.Split(params.ByName("blueprints"), ",")
	if names[0] == "" {
		statusResponseError(writer, http.StatusNotFound)
		return
	}

	blueprints := []entry{}
	for _, name := range names {
		var blueprint blueprint
		if !api.store.getBlueprint(name, &blueprint, nil) {
			statusResponseError(writer, http.StatusNotFound)
			return
		}

		specs := make([]string, len(blueprint.Packages))
		for i, pkg := range blueprint.Packages {
			specs[i] = pkg.Name
			if pkg.Version != "" {
				specs[i] += "-" + pkg.Version
			}
		}

		dependencies, _ := rpmmd.Depsolve(specs...)

		blueprints = append(blueprints, entry{blueprint, dependencies})
	}

	json.NewEncoder(writer).Encode(reply{
		Blueprints: blueprints,
		Errors:     []string{},
	})
}

func (api *API) blueprintsDiffHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	type pack struct {
		Package blueprintPackage `json:"Package"`
	}

	type diff struct {
		New *pack `json:"new"`
		Old *pack `json:"old"`
	}

	type reply struct {
		Diffs []diff `json:"diff"`
	}

	name := params.ByName("blueprint")
	if len(name) == 0 {
		statusResponseError(writer, http.StatusNotFound, "no blueprint name given")
		return
	}
	fromCommit := params.ByName("from")
	if len(fromCommit) == 0 || fromCommit != "NEWEST" {
		statusResponseError(writer, http.StatusNotFound, "invalid from commit ID given")
		return
	}
	toCommit := params.ByName("to")
	if len(toCommit) == 0 || toCommit != "WORKSPACE" {
		statusResponseError(writer, http.StatusNotFound, "invalid to commit ID given")
		return
	}

	// Fetch old and new blueprint details from store and return error if not found
	var oldBlueprint, newBlueprint blueprint
	if !api.store.getBlueprintCommitted(name, &oldBlueprint) || !api.store.getBlueprint(name, &newBlueprint, nil) {
		statusResponseError(writer, http.StatusNotFound)
		return
	}

	newSlice := newBlueprint.Packages
	oldMap := make(map[string]blueprintPackage)
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

func (api *API) blueprintsNewHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		statusResponseError(writer, http.StatusUnsupportedMediaType, "blueprint must be json")
		return
	}

	var blueprint blueprint
	err := json.NewDecoder(request.Body).Decode(&blueprint)
	if err != nil {
		statusResponseError(writer, http.StatusBadRequest, "invalid blueprint: "+err.Error())
		return
	}

	api.store.pushBlueprint(blueprint)

	statusResponseOK(writer)
}

func (api *API) blueprintsWorkspaceHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		statusResponseError(writer, http.StatusUnsupportedMediaType, "blueprint must be json")
		return
	}

	var blueprint blueprint
	err := json.NewDecoder(request.Body).Decode(&blueprint)
	if err != nil {
		statusResponseError(writer, http.StatusBadRequest, "invalid blueprint: "+err.Error())
		return
	}

	api.store.pushBlueprintToWorkspace(blueprint)

	statusResponseOK(writer)
}

func (api *API) blueprintDeleteHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	api.store.deleteBlueprint(params.ByName("blueprint"))
	statusResponseOK(writer)
}

func (api *API) blueprintDeleteWorkspaceHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	api.store.deleteBlueprintFromWorkspace(params.ByName("blueprint"))
	statusResponseOK(writer)
}

func (api *API) composeQueueHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	var reply struct {
		New []interface{} `json:"new"`
		Run []interface{} `json:"run"`
	}

	reply.New = make([]interface{}, 0)
	reply.Run = make([]interface{}, 0)

	json.NewEncoder(writer).Encode(reply)
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
