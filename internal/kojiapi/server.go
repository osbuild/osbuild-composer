//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=kojiapi --generate types,chi-server,client -o openapi.gen.go openapi.yml

// Package kojiapi provides a REST API to build and push images to Koji
package kojiapi

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

// Server represents the state of the koji Server
type Server struct {
	workers     *worker.Server
	rpmMetadata rpmmd.RPMMD
	distros     *distro.Registry
}

// NewServer creates a new koji server
func NewServer(workers *worker.Server, rpmMetadata rpmmd.RPMMD, distros *distro.Registry) *Server {
	server := &Server{
		workers:     workers,
		rpmMetadata: rpmMetadata,
		distros:     distros,
	}
	return server
}

// Serve serves the koji API over the provided listener socket
func (server *Server) Serve(listener net.Listener) error {
	s := http.Server{Handler: Handler(server)}

	err := s.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// PostCompose handles a new /compose POST request
func (server *Server) PostCompose(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		http.Error(w, fmt.Sprintf("Only 'application/json' content type is supported"), http.StatusUnsupportedMediaType)
		return
	}

	var request ComposeRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not parse JSON body"), http.StatusBadRequest)
		return
	}

	d := server.distros.GetDistro(request.Distribution)
	if d == nil {
		http.Error(w, fmt.Sprintf("Unsupported distribution: %s", request.Distribution), http.StatusBadRequest)
		return
	}

	type imageRequest struct {
		manifest distro.Manifest
	}
	imageRequests := make([]imageRequest, len(request.ImageRequests))

	for i, ir := range request.ImageRequests {
		arch, err := d.GetArch(ir.Architecture)
		if err != nil {
			http.Error(w, fmt.Sprintf("Unsupported architecture '%s' for distribution '%s'", ir.Architecture, request.Distribution), http.StatusBadRequest)
			return
		}
		imageType, err := arch.GetImageType(ir.ImageType)
		if err != nil {
			http.Error(w, fmt.Sprintf("Unsupported image type '%s' for %s/%s", ir.ImageType, ir.Architecture, request.Distribution), http.StatusBadRequest)
			return
		}
		repositories := make([]rpmmd.RepoConfig, len(ir.Repositories))
		for j, repo := range ir.Repositories {
			repositories[j].BaseURL = repo.Baseurl
			repositories[j].GPGKey = repo.Gpgkey
		}
		packageSpecs, _ := imageType.BasePackages()
		packages, _, err := server.rpmMetadata.Depsolve(packageSpecs, nil, repositories, d.ModulePlatformID(), arch.Name())
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to depsolve base base packages for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err), http.StatusBadRequest)
			return
		}
		buildPackageSpecs := imageType.BuildPackages()
		buildPackages, _, err := server.rpmMetadata.Depsolve(buildPackageSpecs, nil, repositories, d.ModulePlatformID(), arch.Name())
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to depsolve build packages for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err), http.StatusBadRequest)
			return
		}

		manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, repositories, packages, buildPackages)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get manifest for for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err), http.StatusBadRequest)
			return
		}

		imageRequests[i].manifest = manifest
	}

	var ir imageRequest
	if len(imageRequests) == 1 {
		// NOTE: the store currently does not support multi-image composes
		ir = imageRequests[0]
	} else {
		http.Error(w, fmt.Sprintf("Only single-image composes are currently supported"), http.StatusBadRequest)
		return
	}
	id, err := server.workers.Enqueue(ir.manifest, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to enqueu image build job: %s", err), http.StatusInternalServerError)
		return
	}

	var response ComposeResponse
	response.Id = id.String()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic("Failed to write response")
	}
}

// GetComposeId handles a /compose/{id} GET request
func (server *Server) GetComposeId(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.Context().Value("id").(string))
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid format for parameter id: %s", err), http.StatusBadRequest)
		return
	}

	status, err := server.workers.JobStatus(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Job %s not found: %s", id.String(), err), http.StatusBadRequest)
		return
	}

	response := ComposeStatus{
		Status: status.State.ToString(), // TODO: map the status correctly
		ImageStatuses: []ImageStatus{
			{
				Status: status.State.ToString(), // TODO: map the status correctly
			},
		},
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic("Failed to write response")
	}
}
