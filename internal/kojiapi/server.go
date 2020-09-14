//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=kojiapi --generate types,chi-server,client -o openapi.gen.go openapi.yml

// Package kojiapi provides a REST API to build and push images to Koji
package kojiapi

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

// Server represents the state of the koji Server
type Server struct {
	workers     *worker.Server
	rpmMetadata rpmmd.RPMMD
	distros     *distro.Registry
	kojiServers map[string]koji.GSSAPICredentials
}

// NewServer creates a new koji server
func NewServer(workers *worker.Server, rpmMetadata rpmmd.RPMMD, distros *distro.Registry, kojiServers map[string]koji.GSSAPICredentials) *Server {
	server := &Server{
		workers:     workers,
		rpmMetadata: rpmMetadata,
		distros:     distros,
		kojiServers: kojiServers,
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
		http.Error(w, "Only 'application/json' content type is supported", http.StatusUnsupportedMediaType)
		return
	}

	var request ComposeRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Could not parse JSON body", http.StatusBadRequest)
		return
	}

	d := server.distros.GetDistro(request.Distribution)
	if d == nil {
		http.Error(w, fmt.Sprintf("Unsupported distribution: %s", request.Distribution), http.StatusBadRequest)
		return
	}

	kojiServer, err := url.Parse(request.Koji.Server)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid Koji server: %s", request.Koji.Server), http.StatusBadRequest)
		return
	}
	creds, exists := server.kojiServers[kojiServer.Hostname()]
	if !exists {
		http.Error(w, fmt.Sprintf("Koji server has not been configured: %s", kojiServer.Hostname()), http.StatusBadRequest)
		return

	}

	type imageRequest struct {
		manifest distro.Manifest
		filename string
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
		bp := &blueprint.Blueprint{}
		err = bp.Initialize()
		if err != nil {
			panic("Could not initialize empty blueprint.")
		}
		packageSpecs, _ := imageType.Packages(*bp)
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
		imageRequests[i].filename = imageType.Filename()
	}

	var ir imageRequest
	if len(imageRequests) == 1 {
		// NOTE: the store currently does not support multi-image composes
		ir = imageRequests[0]
	} else {
		http.Error(w, "Only single-image composes are currently supported", http.StatusBadRequest)
		return
	}

	// Koji for some reason needs TLS renegotiation enabled.
	// Clone the default http transport and enable renegotiation.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		Renegotiation: tls.RenegotiateOnceAsClient,
	}

	k, err := koji.NewFromGSSAPI(request.Koji.Server, &creds, transport)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not log into Koji: %v", err), http.StatusBadRequest)
		return
	}

	defer func() {
		err := k.Logout()
		if err != nil {
			log.Printf("koji logout failed: %v", err)
		}
	}()

	buildInfo, err := k.CGInitBuild(request.Name, request.Version, request.Release)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not initialize build with koji: %v", err), http.StatusBadRequest)
		return
	}

	id, err := server.workers.Enqueue(ir.manifest, []*target.Target{
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
		}),
	})
	if err != nil {
		// This is a programming errror.
		panic(err)
	}

	var response ComposeResponse
	response.Id = id.String()
	response.KojiBuildId = buildInfo.BuildID
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic("Failed to write response")
	}
}

func composeStateToStatus(state common.ComposeState) string {
	switch state {
	case common.CFailed:
		return "failure"
	case common.CFinished:
		return "success"
	case common.CRunning:
		return "pending"
	case common.CWaiting:
		return "pending"
	default:
		panic("invalid compose state")
	}
}

func composeStateToImageStatus(state common.ComposeState) string {
	switch state {
	case common.CFailed:
		return "failure"
	case common.CFinished:
		return "success"
	case common.CRunning:
		return "building"
	case common.CWaiting:
		return "pending"
	default:
		panic("invalid compose state")
	}
}

// GetComposeId handles a /compose/{id} GET request
func (server *Server) GetComposeId(w http.ResponseWriter, r *http.Request, id string) {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid format for parameter id: %s", err), http.StatusBadRequest)
		return
	}

	status, err := server.workers.JobStatus(parsedID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Job %s not found: %s", id, err), http.StatusBadRequest)
		return
	}

	response := ComposeStatus{
		Status: composeStateToStatus(status.State),
		ImageStatuses: []ImageStatus{
			{
				Status: composeStateToImageStatus(status.State),
			},
		},
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic("Failed to write response")
	}
}
