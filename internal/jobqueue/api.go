package jobqueue

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/osbuild/osbuild-composer/internal/store"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
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

func methodNotAllowedHandler(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusMethodNotAllowed)
}

func notFoundHandler(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(http.StatusNotFound)
}

func statusResponseOK(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusOK)
}

func statusResponseError(writer http.ResponseWriter, code int, errors ...string) {
	writer.WriteHeader(code)

	for _, err := range errors {
		writer.Write([]byte(err))
	}
}

func (api *API) addJobHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	type requestBody struct {
	}

	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		statusResponseError(writer, http.StatusUnsupportedMediaType)
		return
	}

	var body requestBody
	err := json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		statusResponseError(writer, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	nextJob := api.store.PopJob()

	writer.WriteHeader(http.StatusCreated)
	// FIXME: handle or comment this possible error
	_ = json.NewEncoder(writer).Encode(Job{
		ID:           nextJob.ComposeID,
		ImageBuildID: nextJob.ImageBuildID,
		Distro:       nextJob.Distro,
		Pipeline:     nextJob.Pipeline,
		Targets:      nextJob.Targets,
		OutputType:   nextJob.ImageType,
	})
}

func (api *API) updateJobHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		statusResponseError(writer, http.StatusUnsupportedMediaType)
		return
	}

	id, err := uuid.Parse(params.ByName("job_id"))
	if err != nil {
		statusResponseError(writer, http.StatusBadRequest, "invalid compose id: "+err.Error())
		return
	}

	imageBuildId, err := strconv.Atoi(params.ByName("build_id"))

	if err != nil {
		statusResponseError(writer, http.StatusBadRequest, "invalid image build id: "+err.Error())
		return
	}

	var body JobStatus
	err = json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		statusResponseError(writer, http.StatusBadRequest, "invalid status: "+err.Error())
		return
	}

	err = api.store.UpdateImageBuildInCompose(id, imageBuildId, body.Status, body.Result)
	if err != nil {
		switch err.(type) {
		case *store.NotFoundError:
			statusResponseError(writer, http.StatusNotFound, err.Error())
		case *store.NotPendingError:
			statusResponseError(writer, http.StatusNotFound, err.Error())
		case *store.NotRunningError:
			statusResponseError(writer, http.StatusBadRequest, err.Error())
		case *store.InvalidRequestError:
			statusResponseError(writer, http.StatusBadRequest, err.Error())
		default:
			statusResponseError(writer, http.StatusInternalServerError, err.Error())
		}
		return
	}
	statusResponseOK(writer)
}

func (api *API) addJobImageHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	id, err := uuid.Parse(params.ByName("job_id"))
	if err != nil {
		statusResponseError(writer, http.StatusBadRequest, "invalid compose id: "+err.Error())
		return
	}

	imageBuildId, err := strconv.Atoi(params.ByName("build_id"))

	if err != nil {
		statusResponseError(writer, http.StatusBadRequest, "invalid build id: "+err.Error())
		return
	}

	err = api.store.AddImageToImageUpload(id, imageBuildId, request.Body)

	if err != nil {
		switch err.(type) {
		case *store.NotFoundError:
			statusResponseError(writer, http.StatusNotFound, err.Error())
		case *store.NoLocalTargetError:
			statusResponseError(writer, http.StatusBadRequest, err.Error())
		default:
			statusResponseError(writer, http.StatusInternalServerError, err.Error())
		}
		return
	}

	statusResponseOK(writer)
}
