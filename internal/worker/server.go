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
	"github.com/labstack/echo/v4"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker/api"
)

type Server struct {
	jobs         jobqueue.JobQueue
	echo         *echo.Echo
	artifactsDir string
}

type JobStatus struct {
	State    common.ComposeState
	Queued   time.Time
	Started  time.Time
	Finished time.Time
	Canceled bool
	Result   OSBuildJobResult
}

func NewServer(logger *log.Logger, jobs jobqueue.JobQueue, artifactsDir string) *Server {
	s := &Server{
		jobs:         jobs,
		artifactsDir: artifactsDir,
	}

	s.echo = echo.New()
	s.echo.Binder = binder{}
	s.echo.StdLogger = logger

	api.RegisterHandlers(s.echo, &apiHandlers{s})

	return s
}

func (s *Server) Serve(listener net.Listener) error {
	s.echo.Listener = listener

	err := s.echo.Start("")
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.echo.ServeHTTP(writer, request)
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
		if result.OSBuildOutput != nil && result.OSBuildOutput.Success {
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
		Canceled: canceled,
		Result:   result,
	}, nil
}

func (s *Server) Cancel(id uuid.UUID) error {
	return s.jobs.CancelJob(id)
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

// apiHandlers implements api.ServerInterface - the http api route handlers
// generated from api/openapi.yml. This is a separate object, because these
// handlers should not be exposed on the `Server` object.
type apiHandlers struct {
	server *Server
}

func (h *apiHandlers) GetStatus(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, &statusResponse{
		Status: "OK",
	})
}

func (h *apiHandlers) GetJob(ctx echo.Context, jobId string) error {
	id, err := uuid.Parse(jobId)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse compose id: %v", err)
	}

	status, err := h.server.JobStatus(id)
	if err != nil {
		switch err {
		case jobqueue.ErrNotExist:
			return echo.NewHTTPError(http.StatusNotFound, "job does not exist: %s", id)
		default:
			return err
		}
	}

	return ctx.JSON(http.StatusOK, jobResponse{
		Id:       id,
		Canceled: status.Canceled,
	})
}

func (h *apiHandlers) PostJob(ctx echo.Context) error {
	var body addJobRequest
	err := ctx.Bind(&body)
	if err != nil {
		return err
	}

	var job OSBuildJob
	id, err := h.server.jobs.Dequeue(ctx.Request().Context(), []string{"osbuild"}, &job)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "%v", err)
	}

	return ctx.JSON(http.StatusCreated, addJobResponse{
		Id:       id,
		Manifest: job.Manifest,
		Targets:  job.Targets,
	})
}

func (h *apiHandlers) UpdateJob(ctx echo.Context, jobId string) error {
	id, err := uuid.Parse(jobId)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse compose id: %v", err)
	}

	var body updateJobRequest
	err = ctx.Bind(&body)
	if err != nil {
		return err
	}

	// The jobqueue doesn't support setting the status before a job is
	// finished. This branch should never be hit, because the worker
	// doesn't attempt this. Change the API to remove this awkwardness.
	if body.Status != common.IBFinished && body.Status != common.IBFailed {
		return echo.NewHTTPError(http.StatusBadRequest, "setting status of a job to waiting or running is not supported")
	}

	err = h.server.jobs.FinishJob(id, OSBuildJobResult{OSBuildOutput: body.Result})
	if err != nil {
		switch err {
		case jobqueue.ErrNotExist:
			return echo.NewHTTPError(http.StatusNotFound, "job does not exist: %s", id)
		case jobqueue.ErrNotRunning:
			return echo.NewHTTPError(http.StatusBadRequest, "job is not running: %s", id)
		default:
			return err
		}
	}

	return ctx.JSON(http.StatusOK, updateJobResponse{})
}

func (h *apiHandlers) PostJobArtifact(ctx echo.Context, jobId string, name string) error {
	id, err := uuid.Parse(jobId)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse compose id: %v", err)
	}

	request := ctx.Request()

	if h.server.artifactsDir == "" {
		_, err := io.Copy(ioutil.Discard, request.Body)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "error discarding artifact: %v", err)
		}
		return ctx.NoContent(http.StatusOK)
	}

	err = os.Mkdir(path.Join(h.server.artifactsDir, id.String()), 0700)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot create artifact directory: %v", err)
	}

	f, err := os.Create(path.Join(h.server.artifactsDir, id.String(), name))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot create artifact file: %v", err)
	}

	_, err = io.Copy(f, request.Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error writing artifact file: %v", err)
	}

	return ctx.NoContent(http.StatusOK)
}

// A simple echo.Binder(), which only accepts application/json, but is more
// strict than echo's DefaultBinder. It does not handle binding query
// parameters either.
type binder struct{}

func (b binder) Bind(i interface{}, ctx echo.Context) error {
	request := ctx.Request()

	contentType := request.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		return echo.NewHTTPError(http.StatusUnsupportedMediaType, "request must be json-encoded")
	}

	err := json.NewDecoder(request.Body).Decode(i)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse request body: %v", err)
	}

	return nil
}
