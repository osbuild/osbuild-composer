package worker

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

type Server struct {
	logger *log.Logger
	store  *store.Store
	router *httprouter.Router
}

func NewServer(logger *log.Logger, store *store.Store) *Server {
	s := &Server{
		logger: logger,
		store:  store,
	}

	s.router = httprouter.New()
	s.router.RedirectTrailingSlash = false
	s.router.RedirectFixedPath = false
	s.router.MethodNotAllowed = http.HandlerFunc(methodNotAllowedHandler)
	s.router.NotFound = http.HandlerFunc(notFoundHandler)

	s.router.POST("/job-queue/v1/jobs", s.addJobHandler)
	s.router.PATCH("/job-queue/v1/jobs/:job_id/builds/:build_id", s.updateJobHandler)
	s.router.POST("/job-queue/v1/jobs/:job_id/builds/:build_id/image", s.addJobImageHandler)

	return s
}

func (s *Server) Serve(listener net.Listener) error {
	server := http.Server{Handler: s}

	err := server.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if s.logger != nil {
		log.Println(request.Method, request.URL.Path)
	}

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	s.router.ServeHTTP(writer, request)
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

func (s *Server) addJobHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
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

	nextJob := s.store.PopJob()

	writer.WriteHeader(http.StatusCreated)
	// FIXME: handle or comment this possible error
	_ = json.NewEncoder(writer).Encode(addJobResponse{
		ComposeID:    nextJob.ComposeID,
		ImageBuildID: nextJob.ImageBuildID,
		Manifest:     nextJob.Manifest,
		Targets:      nextJob.Targets,
	})
}

func (s *Server) updateJobHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
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

	err = s.store.UpdateImageBuildInCompose(id, imageBuildId, body.Status, body.Result)
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

func (s *Server) addJobImageHandler(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
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

	err = s.store.AddImageToImageUpload(id, imageBuildId, request.Body)

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
