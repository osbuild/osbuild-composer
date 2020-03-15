// Package rcm provides alternative HTTP API to Weldr.
// It's primary use case is for the RCM team. As such it is driven solely by their requirements.
package rcm

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/store"
)

// API encapsulates RCM-specific API that is exposed over a separate TCP socket
type API struct {
	logger *log.Logger
	store  *store.Store
	router *httprouter.Router
	// rpmMetadata is an interface to dnf-json and we include it here so that we can
	// mock it in the unit tests
	rpmMetadata rpmmd.RPMMD
}

// New creates new RCM API
func New(logger *log.Logger, store *store.Store, rpmMetadata rpmmd.RPMMD) *API {
	api := &API{
		logger:      logger,
		store:       store,
		router:      httprouter.New(),
		rpmMetadata: rpmMetadata,
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

func (api *API) submit(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	// Check some basic HTTP parameters
	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	type Repository struct {
		URL string `json:"url"`
	}

	// JSON structure expected from the client
	var composeRequest struct {
		Distribution  common.Distribution   `json:"distribution"`
		ImageTypes    []common.ImageType    `json:"image_types"`
		Architectures []common.Architecture `json:"architectures"`
		Repositories  []Repository          `json:"repositories"`
	}
	// JSON structure with error message
	var errorReason struct {
		Error string `json:"error_reason"`
	}
	// Parse and verify the structure
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&composeRequest)
	if err != nil || len(composeRequest.Architectures) == 0 || len(composeRequest.Repositories) == 0 || len(composeRequest.ImageTypes) == 0 {
		writer.WriteHeader(http.StatusBadRequest)
		errors := []string{}
		if err != nil {
			errors = append(errors, err.Error())
		}
		if len(composeRequest.ImageTypes) == 0 {
			errors = append(errors, "input must specify an image type")
		} else if len(composeRequest.ImageTypes) != 1 {
			errors = append(errors, "multiple image types are not yet supported")
		}
		if len(composeRequest.Architectures) == 0 {
			errors = append(errors, "input must specify an architecture")
		} else if len(composeRequest.Architectures) != 1 {
			errors = append(errors, "multiple architectures are not yet supported")
		}
		if len(composeRequest.Repositories) == 0 {
			errors = append(errors, "input must specify repositories")
		}
		errorReason.Error = strings.Join(errors, ", ")
		err = json.NewEncoder(writer).Encode(errorReason)
		if err != nil {
			// JSON encoding is clearly our fault.
			panic("Failed to encode errors in RCM API. This is a bug.")
		}
		return
	}

	// Create repo configurations from the URLs in the request. Use made up repo id and name, because
	// we don't want to bother clients of this API with details like this
	repoConfigs := []rpmmd.RepoConfig{}
	for n, repo := range composeRequest.Repositories {
		repoConfigs = append(repoConfigs, rpmmd.RepoConfig{
			Id:        fmt.Sprintf("repo-%d", n),
			BaseURL:   repo.URL,
			IgnoreSSL: false,
		})
	}

	// Image requests are derived from requested image types. All of them are uploaded to Koji, because
	// this API is only for RCM.
	requestedImages := []common.ImageRequest{}
	for _, imgType := range composeRequest.ImageTypes {
		requestedImages = append(requestedImages, common.ImageRequest{
			ImgType: imgType,
			// TODO: use koji upload type as soon as it is available
			UpTarget: []common.UploadTarget{},
		})
	}

	modulePlatformID, err := composeRequest.Distribution.ModulePlatformID()
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_, err := writer.Write([]byte(err.Error()))
		if err != nil {
			panic("Failed to write response")
		}
	}

	// map( repo-id => checksum )
	_, checksums, err := api.rpmMetadata.FetchMetadata(repoConfigs, modulePlatformID)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_, err := writer.Write([]byte(err.Error()))
		if err != nil {
			panic("Failed to write response")
		}
	}

	// Push the requested compose to the store
	composeUUID := uuid.New()
	// nil is used as an upload target, because LocalTarget is already used in the PushCompose function
	err = api.store.PushComposeRequest(store.ComposeRequest{
		Blueprint:       blueprint.Blueprint{},
		ComposeID:       composeUUID,
		Distro:          composeRequest.Distribution,
		Arch:            composeRequest.Architectures[0],
		Repositories:    repoConfigs,
		Checksums:       checksums,
		RequestedImages: requestedImages,
	})
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
	reply.UUID = composeUUID
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
	compose, exists := api.store.GetCompose(id)
	if !exists {
		writer.WriteHeader(http.StatusBadRequest)
		errorReason.Error = "Compose UUID does not exist"
		// TODO: handle error
		_ = json.NewEncoder(writer).Encode(errorReason)
		return
	}

	// JSON structure with success response
	var reply struct {
		Status string `json:"status"`
	}

	// TODO: return per-job status like Koji does (requires changes in the store)
	reply.Status = compose.GetState().ToString()
	// TODO: handle error
	_ = json.NewEncoder(writer).Encode(reply)
}
