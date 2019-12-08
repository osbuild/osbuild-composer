// Package rcm provides alternative HTTP API to Weldr.
// It's primary use case is for the RCM team. As such it is driven solely by their requirements.
package rcm

import (
	"encoding/json"
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
}

// New creates new RCM API
func New(logger *log.Logger, store *store.Store) *API {
	api := &API{
		logger: logger,
		store:  store,
		router: httprouter.New(),
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
		URL      string `json:"url"`
		Checksum string `json:"checksum"`
	}

	// JSON structure expected from the client
	var composeRequest struct {
		Distribution  string       `json:"distribution"`
		ImageTypes    []string     `json:"image_types"`
		Architectures []string     `json:"architectures"`
		Repositories  []Repository `json:"repositories"`
	}
	// JSON structure with error message
	var errorReason struct {
		Error string `json:"error_reason"`
	}
	// Parse and verify the structure
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&composeRequest)
	if err != nil || composeRequest.Distribution == "" || len(composeRequest.Architectures) == 0 || len(composeRequest.Repositories) == 0 {
		writer.WriteHeader(http.StatusBadRequest)
		errors := []string{}
		if err != nil {
			errors = append(errors, err.Error())
		}
		if composeRequest.Distribution == "" {
			errors = append(errors, "input must specify a distribution")
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
		json.NewEncoder(writer).Encode(errorReason)
		return
	}

	// Push the requested compose to the store
	composeUUID := uuid.New()
	// nil is used as an upload target, because LocalTarget is already used in the PushCompose function
	err = api.store.PushCompose(composeUUID, &blueprint.Blueprint{}, make(map[string]string), composeRequest.Architectures[0], composeRequest.ImageTypes[0], 0, nil)
	if err != nil {
		if api.logger != nil {
			api.logger.Println("RCM API failed to push compose:", err)
		}
		writer.WriteHeader(http.StatusInternalServerError)
		errorReason.Error = "failed to push compose: " + err.Error()
		json.NewEncoder(writer).Encode(errorReason)
		return
	}

	// Create the response JSON structure
	var reply struct {
		UUID uuid.UUID `json:"compose_id"`
	}
	reply.UUID = composeUUID
	json.NewEncoder(writer).Encode(reply)
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
		json.NewEncoder(writer).Encode(errorReason)
		return
	}

	// Check that the compose exists
	compose, exists := api.store.GetCompose(id)
	if !exists {
		writer.WriteHeader(http.StatusBadRequest)
		errorReason.Error = "Compose UUID does not exist"
		json.NewEncoder(writer).Encode(errorReason)
		return
	}

	// JSON structure with success response
	var reply struct {
		Status string `json:"status"`
	}

	// TODO: return per-job status like Koji does (requires changes in the store)
	reply.Status = compose.QueueStatus
	json.NewEncoder(writer).Encode(reply)
}
