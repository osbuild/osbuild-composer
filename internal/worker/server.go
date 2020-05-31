package worker

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/target"
)

type Server struct {
	logger       *log.Logger
	jobs         jobqueue.JobQueue
	router       *httprouter.Router
	artifactsDir string
}

type JobStatus struct {
	State    common.ComposeState
	Queued   time.Time
	Started  time.Time
	Finished time.Time
	Result   OSBuildJobResult
}

func NewServer(logger *log.Logger, jobs jobqueue.JobQueue, artifactsDir string) *Server {
	s := &Server{
		logger:       logger,
		jobs:         jobs,
		artifactsDir: artifactsDir,
	}

	s.router = httprouter.New()
	s.router.RedirectTrailingSlash = false
	s.router.RedirectFixedPath = false
	s.router.MethodNotAllowed = http.HandlerFunc(methodNotAllowedHandler)
	s.router.NotFound = http.HandlerFunc(notFoundHandler)

	// Add a basic status handler for checking if osbuild-composer is alive.
	s.router.GET("/status", s.statusHandler)

	// Add handlers for managing jobs.
	s.router.POST("/job-queue/v1/jobs", s.addJobHandler)
	s.router.PATCH("/job-queue/v1/jobs/:job_id", s.updateJobHandler)
	s.router.POST("/job-queue/v1/jobs/:job_id/artifacts/:name", s.addJobImageHandler)

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

func (s *Server) Enqueue(manifest distro.Manifest, targets []*target.Target) (uuid.UUID, error) {
	job := OSBuildJob{
		Manifest: manifest,
		Targets:  targets,
	}

	return s.jobs.Enqueue("osbuild", job, nil)
}

func (s *Server) JobStatus(id uuid.UUID) (*JobStatus, error) {
	var canceled bool
	var result OSBuildJobResult

	queued, started, finished, canceled, err := s.jobs.JobStatus(id, &result)
	if err != nil {
		return nil, err
	}
	state := common.CWaiting
	if canceled {
		state = common.CFailed
	} else if !finished.IsZero() {
		if result.OSBuildOutput.Success {
			state = common.CFinished
		} else {
			state = common.CFailed
		}
	} else if !started.IsZero() {
		state = common.CRunning
	}

	return &JobStatus{
		State:    state,
		Queued:   queued,
		Started:  started,
		Finished: finished,
		Result:   result,
	}, nil
}

// Provides access to artifacts of a job. Returns an io.Reader for the artifact
// and the artifact's size.
func (s *Server) JobArtifact(id uuid.UUID, name string) (io.Reader, int64, error) {
	status, err := s.JobStatus(id)
	if err != nil {
		return nil, 0, err
	}

	if status.Finished.IsZero() {
		return nil, 0, fmt.Errorf("Cannot access artifacts before job is finished: %s", id)
	}

	p := path.Join(s.artifactsDir, id.String(), name)
	f, err := os.Open(p)
	if err != nil {
		return nil, 0, fmt.Errorf("Error accessing artifact %s for job %s: %v", name, id, err)
	}

	info, err := f.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("Error getting size of artifact %s for job %s: %v", name, id, err)
	}

	return f, info.Size(), nil
}

// Deletes all artifacts for job `id`.
func (s *Server) DeleteArtifacts(id uuid.UUID) error {
	status, err := s.JobStatus(id)
	if err != nil {
		return err
	}

	if status.Finished.IsZero() {
		return fmt.Errorf("Cannot delete artifacts before job is finished: %s", id)
	}

	return os.RemoveAll(path.Join(s.artifactsDir, id.String()))
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

func (s *Server) statusHandler(writer http.ResponseWriter, request *http.Request, _ httprouter.Params) {
	writer.WriteHeader(http.StatusOK)

	// Send back a status message.
	_ = json.NewEncoder(writer).Encode(&statusResponse{
		Status: "OK",
	})
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

	var job OSBuildJob
	id, err := s.jobs.Dequeue(request.Context(), []string{"osbuild"}, &job)
	if err != nil {
		jsonErrorf(writer, http.StatusInternalServerError, "%v", err)
		return
	}

	writer.WriteHeader(http.StatusCreated)
	// FIXME: handle or comment this possible error
	_ = json.NewEncoder(writer).Encode(addJobResponse{
		Id:       id,
		Manifest: job.Manifest,
		Targets:  job.Targets,
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

	var body updateJobRequest
	err = json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		jsonErrorf(writer, http.StatusBadRequest, "cannot parse request body: %v", err)
		return
	}

	// The jobqueue doesn't support setting the status before a job is
	// finished. This branch should never be hit, because the worker
	// doesn't attempt this. Change the API to remove this awkwardness.
	if body.Status != common.IBFinished && body.Status != common.IBFailed {
		jsonErrorf(writer, http.StatusBadRequest, "setting status of a job to waiting or running is not supported")
		return
	}

	err = s.jobs.FinishJob(id, OSBuildJobResult{OSBuildOutput: body.Result})
	if err != nil {
		switch err {
		case jobqueue.ErrNotExist:
			jsonErrorf(writer, http.StatusNotFound, "job does not exist: %s", id)
		case jobqueue.ErrNotRunning:
			jsonErrorf(writer, http.StatusBadRequest, "job is not running: %s", id)
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

	name := params.ByName("name")
	if name == "" {
		jsonErrorf(writer, http.StatusBadRequest, "invalid artifact name")
		return
	}

	if s.artifactsDir == "" {
		_, err := io.Copy(ioutil.Discard, request.Body)
		if err != nil {
			jsonErrorf(writer, http.StatusInternalServerError, "error discarding artifact: %v", err)
		}
		return
	}

	err = os.Mkdir(path.Join(s.artifactsDir, id.String()), 0700)
	if err != nil {
		jsonErrorf(writer, http.StatusInternalServerError, "cannot create artifact directory: %v", err)
		return
	}

	f, err := os.Create(path.Join(s.artifactsDir, id.String(), name))
	if err != nil {
		jsonErrorf(writer, http.StatusInternalServerError, "cannot create artifact file: %v", err)
		return
	}

	_, err = io.Copy(f, request.Body)
	if err != nil {
		jsonErrorf(writer, http.StatusInternalServerError, "error writing artifact file: %v", err)
		return
	}
}
