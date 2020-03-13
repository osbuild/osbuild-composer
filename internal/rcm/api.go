// Package rcm RCM API for osbuild-composer
//
// It's primary use case is for the RCM team. As such it is driven solely by their requirements.
//
// Version: 1
//
// swagger:meta
package rcm

import (
	"encoding/json"
	"fmt"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"log"
	"net"
	"net/http"
	"strings"

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

// submitReply represents the response after submitting a compose
//
// swagger:model
type submitReply struct {
	// the UUID of the compose
	//
	// In the future this will be replaced with Koji ID.
	//
	// required: true
	UUID uuid.UUID `json:"compose_id"`
}

// statusReply represents the response when getting a status of a running compose
//
// swagger:model
type statusReply struct {
	// the status of running compose
	//
	// required: true
	Status string `json:"status"`
}

// errorReason represents the response in case of any error
//
// swagger:model
type errorReason struct {
	// a string describing what happened
	//
	// required: true
	Error string `json:"error_reason"`
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

	// swagger:route POST /v1/compose submit
	//
	// Submit requests for composes.
	//
	//     Consumes:
	//     - application/json
	//
	//     Produces:
	//     - application/json
	//
	//     Schemes: http
	//
	//     Responses:
	//       default: errorReason
	//       200: submitReply
	api.router.POST("/v1/compose", api.submit)

	// swagger:operation GET /v1/compose/{uuid} status
	//
	// Check current status of a running compose.
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: uuid
	//   in: path
	//   description: uuid to display status for
	//   required: true
	//   type: string
	// responses:
	//   '200':
	//     description: status of a running compose
	//     schema:
	//       type: object
	//       items:
	//         "$ref": "#/definitions/statusReply"
	//   default:
	//     description: unexpected error
	//     schema:
	//       "$ref": "#/definitions/errorReason"
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
			Name:      fmt.Sprintf("repo-%d", n),
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

	var reply submitReply
	reply.UUID = composeUUID
	// TODO: handle error
	_ = json.NewEncoder(writer).Encode(reply)
}

func (api *API) status(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	// JSON structure in case of error
	var errorReason errorReason
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
	var reply statusReply

	// TODO: return per-job status like Koji does (requires changes in the store)
	reply.Status = compose.GetState().ToString()
	// TODO: handle error
	_ = json.NewEncoder(writer).Encode(reply)
}
