// Package ComposerKoji provides a REST API to build and push images to Koji
package ComposerKoji

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
)

// API represents the state of the composer-koji API
type API struct {
	store       *store.Store
	rpmMetadata rpmmd.RPMMD
	distros     *distro.Registry
}

// New creates a new composer-koji API
func New(store *store.Store, rpmMetadata rpmmd.RPMMD, distros *distro.Registry) *API {
	api := &API{
		store:       store,
		rpmMetadata: rpmMetadata,
		distros:     distros,
	}
	return api
}

// Serve serves the composer-koji API over the provided listener socket
func (api *API) Serve(listener net.Listener) error {
	server := http.Server{Handler: Handler(api)}

	err := server.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// PostCompose handles a new /v1/compose POST request
func (api *API) PostCompose(w http.ResponseWriter, r *http.Request) {
	var request ComposeRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	d := api.distros.GetDistro(request.Distribution)
	if d == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	type imageRequest struct {
		imageType     distro.ImageType
		repositories  []rpmmd.RepoConfig
		packages      []rpmmd.PackageSpec
		buildPackages []rpmmd.PackageSpec
	}
	imageRequests := make([]imageRequest, len(request.ImageRequests))

	for i, ir := range request.ImageRequests {
		arch, err := d.GetArch(ir.Architecture)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		imageType, err := arch.GetImageType(ir.ImageType)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		repositories := make([]rpmmd.RepoConfig, len(ir.Repositories))
		for j, repo := range ir.Repositories {
			repositories[j].BaseURL = repo.Baseurl
		}
		packageSpecs, _ := imageType.BasePackages()
		packages, _, err := api.rpmMetadata.Depsolve(packageSpecs, nil, repositories, d.ModulePlatformID(), arch.Name())
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		buildPackageSpecs := imageType.BuildPackages()
		buildPackages, _, err := api.rpmMetadata.Depsolve(buildPackageSpecs, nil, repositories, d.ModulePlatformID(), arch.Name())

		imageRequests[i].imageType = imageType
		imageRequests[i].repositories = repositories
		imageRequests[i].packages = packages
		imageRequests[i].buildPackages = buildPackages
	}

	var ir imageRequest
	if len(imageRequests) == 1 {
		// NOTE: the store currently does not support multi-image composes
		ir = imageRequests[0]
	} else {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	id, err := api.store.PushCompose(ir.imageType, nil, ir.repositories, ir.packages, ir.buildPackages, 0, nil)

	var response ComposeResponse
	response.Id = id.String()
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetComposeId handles a /v1/compose/{id} GET request
func (api *API) GetComposeId(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.Context().Value("id").(string))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	compose, ok := api.store.GetCompose(id)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var response ComposeStatus
	response.Status = compose.GetState().ToString()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
