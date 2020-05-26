// Package rcm provides alternative HTTP API to Weldr.
// It's primary use case is for the RCM team. As such it is driven solely by their requirements.
package rcm

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/worker"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/osbuild/osbuild-composer/internal/distro"
)

// API encapsulates RCM-specific API that is exposed over a separate TCP socket
type API struct {
	logger  *log.Logger
	workers *worker.Server
	router  *httprouter.Router
	// rpmMetadata is an interface to dnf-json and we include it here so that we can
	// mock it in the unit tests
	rpmMetadata rpmmd.RPMMD
	distros     *distro.Registry
}

// New creates new RCM API
func New(logger *log.Logger, workers *worker.Server, rpmMetadata rpmmd.RPMMD, distros *distro.Registry) *API {
	api := &API{
		logger:      logger,
		workers:     workers,
		router:      httprouter.New(),
		rpmMetadata: rpmMetadata,
		distros:     distros,
	}

	api.router.RedirectTrailingSlash = false
	api.router.RedirectFixedPath = false
	api.router.MethodNotAllowed = http.HandlerFunc(methodNotAllowedHandler)
	api.router.NotFound = http.HandlerFunc(notFoundHandler)

	api.router.POST("/v1/compose", api.submit)
	api.router.GET("/v1/compose/:uuid", api.status)

	return api
}

// Serve serves the RCM API over the provided listener socket
func (api *API) Serve(listener net.Listener) error {
	server := http.Server{Handler: api}

	err := server.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// ServeHTTP logs the request, sets content-type, and forwards the request to appropriate handler
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

// Depsolves packages and build packages for building an image for a given
// distro, in the given architecture
func depsolve(rpmmd rpmmd.RPMMD, distro distro.Distro, imageType distro.ImageType, repos []rpmmd.RepoConfig, arch distro.Arch) ([]rpmmd.PackageSpec, []rpmmd.PackageSpec, error) {
	specs, excludeSpecs := imageType.BasePackages()
	packages, _, err := rpmmd.Depsolve(specs, excludeSpecs, repos, distro.ModulePlatformID(), arch.Name())
	if err != nil {
		return nil, nil, fmt.Errorf("RPMMD.Depsolve: %v", err)
	}

	specs = imageType.BuildPackages()
	buildPackages, _, err := rpmmd.Depsolve(specs, nil, repos, distro.ModulePlatformID(), arch.Name())
	if err != nil {
		return nil, nil, fmt.Errorf("RPMMD.Depsolve: %v", err)
	}

	return packages, buildPackages, err
}

func (api *API) submit(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	// Check some basic HTTP parameters
	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	type repository struct {
		BaseURL    string `json:"baseurl,omitempty"`
		Metalink   string `json:"metalink,omitempty"`
		MirrorList string `json:"mirrorlist,omitempty"`
		GPGKey     string `json:"gpgkey,omitempty"`
	}

	type imageBuild struct {
		Distribution string       `json:"distribution"`
		Architecture string       `json:"architecture"`
		ImageType    string       `json:"image_type"`
		Repositories []repository `json:"repositories"`
	}

	// JSON structure expected from the client
	var composeRequest struct {
		ImageBuilds []imageBuild `json:"image_builds"`
	}
	// JSON structure with error message
	var errorReason struct {
		Error string `json:"error_reason"`
	}
	// Parse and verify the structure
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&composeRequest)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		err = json.NewEncoder(writer).Encode(err.Error())
		if err != nil {
			panic("Failed to write response")
		}
		return
	}

	if len(composeRequest.ImageBuilds) != 1 {
		writer.WriteHeader(http.StatusBadRequest)
		_, err := writer.Write([]byte("unsupported number of image builds"))
		if err != nil {
			panic("Failed to write response")
		}
		return
	}

	buildRequest := composeRequest.ImageBuilds[0]

	d := api.distros.GetDistro(buildRequest.Distribution)
	if d == nil {
		writer.WriteHeader(http.StatusBadRequest)
		_, err := writer.Write([]byte("unknown distro"))
		if err != nil {
			panic("Failed to write response")
		}
		return
	}

	arch, err := d.GetArch(buildRequest.Architecture)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_, err := writer.Write([]byte("unknown architecture for distro"))
		if err != nil {
			panic("Failed to write response")
		}
		return
	}

	imageType, err := arch.GetImageType(buildRequest.ImageType)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_, err := writer.Write([]byte("unknown image type for distro and architecture"))
		if err != nil {
			panic("Failed to write response")
		}
		return
	}

	// Create repo configurations from the URLs in the request.
	repoConfigs := []rpmmd.RepoConfig{}
	for n, repo := range buildRequest.Repositories {
		repoConfigs = append(repoConfigs, rpmmd.RepoConfig{
			Name:       fmt.Sprintf("repo-%d", n),
			BaseURL:    repo.BaseURL,
			Metalink:   repo.Metalink,
			MirrorList: repo.MirrorList,
			GPGKey:     repo.GPGKey,
		})
	}

	packages, buildPackages, err := depsolve(api.rpmMetadata, d, imageType, repoConfigs, arch)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_, err := writer.Write([]byte(err.Error()))
		if err != nil {
			panic("Failed to write response")
		}
		return
	}

	manifest, err := imageType.Manifest(nil,
		distro.ImageOptions{
			Size: imageType.Size(0),
		},
		repoConfigs,
		packages,
		buildPackages)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_, err := writer.Write([]byte(err.Error()))
		if err != nil {
			panic("Failed to write response")
		}
		return
	}

	composeID, err := api.workers.Enqueue(manifest, nil)
	if err != nil {
		if api.logger != nil {
			api.logger.Println("RCM API failed to push compose:", err)
		}
		writer.WriteHeader(http.StatusBadRequest)
		errorReason.Error = "failed to push compose: " + err.Error()
		// TODO: handle error
		_ = json.NewEncoder(writer).Encode(errorReason)
		return
	}

	// Create the response JSON structure
	var reply struct {
		UUID uuid.UUID `json:"compose_id"`
	}
	reply.UUID = composeID
	// TODO: handle error
	_ = json.NewEncoder(writer).Encode(reply)
}

func (api *API) status(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	// JSON structure in case of error
	var errorReason struct {
		Error string `json:"error_reason"`
	}
	// Check that the input is a valid UUID
	uuidParam := params.ByName("uuid")
	id, err := uuid.Parse(uuidParam)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		errorReason.Error = "Malformed UUID"
		// TODO: handle error
		_ = json.NewEncoder(writer).Encode(errorReason)
		return
	}

	// Check that the compose exists
	status, err := api.workers.JobStatus(id)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		errorReason.Error = err.Error()
		// TODO: handle error
		_ = json.NewEncoder(writer).Encode(errorReason)
		return
	}

	// JSON structure with success response
	type reply struct {
		Status string `json:"status"`
	}

	// TODO: handle error
	_ = json.NewEncoder(writer).Encode(reply{Status: status.State.ToString()})
}
