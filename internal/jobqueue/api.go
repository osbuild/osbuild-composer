package jobqueue

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"

	"github.com/osbuild/osbuild-composer/internal/store"
)

type API struct {
	logger *log.Logger
	store  *store.Store
	router *httprouter.Router
}

func New(logger *log.Logger, store *store.Store) *API {
	api := &API{
		logger: logger,
		store:  store,
	}

	api.router = httprouter.New()
	api.router.RedirectTrailingSlash = false
	api.router.RedirectFixedPath = false
	api.router.MethodNotAllowed = http.HandlerFunc(methodNotAllowedHandler)
	api.router.NotFound = http.HandlerFunc(notFoundHandler)

	api.router.POST("/job-queue/v1/jobs", api.addJobHandler)
	api.router.PATCH("/job-queue/v1/jobs/:job_id/builds/:build_id", api.updateJobHandler)
	api.router.POST("/job-queue/v1/jobs/:job_id/builds/:build_id/image", api.addJobImageHandler)

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

// jsonErrorf() is similar to http.Error(), but returns the message in a json
// object with a "message" field.
func jsonErrorf(writer http.ResponseWriter, code int, message string, args ...interface{}) {
	writer.WriteHeader(code)

	// ignore error, because we cannot do anything useful with it
	_ = json.NewEncoder(writer).Encode(&errorResponse{
		Message: fmt.Sprintf(message, args...),
	})
}

func methodNotAllowedHandler(writer http.ResponseWriter, request *http.Request) {
	jsonErrorf(writer, http.StatusMethodNotAllowed, "method not allowed")
}

func notFoundHandler(writer http.ResponseWriter, request *http.Request) {
	jsonErrorf(writer, http.StatusNotFound, "not found")
}

func (api *API) addJobHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		jsonErrorf(writer, http.StatusUnsupportedMediaType, "request must contain application/json data")
		return
	}

	var body addJobRequest
	err := json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		jsonErrorf(writer, http.StatusBadRequest, "%v", err)
		return
	}

	nextJob := api.store.PopJob()

	writer.WriteHeader(http.StatusCreated)
	// FIXME: handle or comment this possible error
	_ = json.NewEncoder(writer).Encode(addJobResponse{
		ComposeID:    nextJob.ComposeID,
		ImageBuildID: nextJob.ImageBuildID,
		Manifest:     nextJob.Manifest,
		Targets:      nextJob.Targets,
	})
}

func (api *API) updateJobHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		jsonErrorf(writer, http.StatusUnsupportedMediaType, "request must contain application/json data")
		return
	}

	id, err := uuid.Parse(params.ByName("job_id"))
	if err != nil {
		jsonErrorf(writer, http.StatusBadRequest, "cannot parse compose id: %v", err)
		return
	}

	imageBuildId, err := strconv.Atoi(params.ByName("build_id"))

	if err != nil {
		jsonErrorf(writer, http.StatusBadRequest, "cannot parse image build id: %v", err)
		return
	}

	var body updateJobRequest
	err = json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		jsonErrorf(writer, http.StatusBadRequest, "cannot parse request body: %v", err)
		return
	}

	err = api.store.UpdateImageBuildInCompose(id, imageBuildId, body.Status, body.Result)
	if err != nil {
		switch err.(type) {
		case *store.NotFoundError, *store.NotPendingError:
			jsonErrorf(writer, http.StatusNotFound, "%v", err)
		case *store.NotRunningError, *store.InvalidRequestError:
			jsonErrorf(writer, http.StatusBadRequest, "%v", err)
		default:
			jsonErrorf(writer, http.StatusInternalServerError, "%v", err)
		}
		return
	}

	_ = json.NewEncoder(writer).Encode(updateJobResponse{})
}

func (api *API) addJobImageHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	id, err := uuid.Parse(params.ByName("job_id"))
	if err != nil {
		jsonErrorf(writer, http.StatusBadRequest, "cannot parse compose id: %v", err)
		return
	}

	imageBuildId, err := strconv.Atoi(params.ByName("build_id"))

	if err != nil {
		jsonErrorf(writer, http.StatusBadRequest, "cannot parse image build id: %v", err)
		return
	}

	err = api.store.AddImageToImageUpload(id, imageBuildId, request.Body)

	if err != nil {
		switch err.(type) {
		case *store.NotFoundError:
			jsonErrorf(writer, http.StatusNotFound, "%v", err)
		case *store.NoLocalTargetError:
			jsonErrorf(writer, http.StatusBadRequest, "%v", err)
		default:
			jsonErrorf(writer, http.StatusInternalServerError, "%v", err)
		}
		return
	}
}
