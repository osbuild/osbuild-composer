package jobqueue

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"osbuild-composer/internal/job"
	"osbuild-composer/internal/pipeline"
	"osbuild-composer/internal/store"
	"osbuild-composer/internal/target"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
)

type API struct {
	pendingJobs <-chan job.Job

	logger *log.Logger
	store  *store.Store
	router *httprouter.Router
}

func New(logger *log.Logger, store *store.Store, jobs <-chan job.Job) *API {
	api := &API{
		logger:      logger,
		store:       store,
		pendingJobs: jobs,
	}

	api.router = httprouter.New()
	api.router.RedirectTrailingSlash = false
	api.router.RedirectFixedPath = false
	api.router.MethodNotAllowed = http.HandlerFunc(methodNotAllowedHandler)
	api.router.NotFound = http.HandlerFunc(notFoundHandler)

	api.router.POST("/job-queue/v1/jobs", api.addJobHandler)
	api.router.PATCH("/job-queue/v1/jobs/:id", api.updateJobHandler)

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
}

func (api *API) addJobHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	type requestBody struct {
	}
	type replyBody struct {
		ID       uuid.UUID          `json:"id"`
		Pipeline *pipeline.Pipeline `json:"pipeline"`
		Targets  []*target.Target   `json:"targets"`
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

	nextJob := <-api.pendingJobs

	writer.WriteHeader(http.StatusCreated)
	json.NewEncoder(writer).Encode(replyBody{nextJob.ComposeID, nextJob.Pipeline, nextJob.Targets})
}

func (api *API) updateJobHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	type requestBody struct {
		Status string `json:"status"`
	}

	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		statusResponseError(writer, http.StatusUnsupportedMediaType)
		return
	}

	id, err := uuid.Parse(params.ByName("id"))
	if err != nil {
		statusResponseError(writer, http.StatusBadRequest, "invalid compose id: "+err.Error())
		return
	}

	var body requestBody
	err = json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		statusResponseError(writer, http.StatusBadRequest, "invalid status: "+err.Error())
	}

	api.store.UpdateCompose(id, body.Status)
	statusResponseOK(writer)
}
