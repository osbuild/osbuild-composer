package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"sync"
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
	server       *http.Server
	artifactsDir string

	// Currently running jobs. Workers are not handed job ids, but
	// independent tokens which serve as an indirection. This enables
	// race-free uploading of artifacts and makes restarting composer more
	// robust (workers from an old run cannot report results for jobs
	// composer thinks are not running).
	// This map maps these tokens to job ids. Artifacts are stored in
	// `$STATE_DIRECTORY/artifacts/tmp/$TOKEN` while the worker is running,
	// and renamed to `$STATE_DIRECTORY/artifacts/$JOB_ID` once the job is
	// reported as done.
	running      map[uuid.UUID]uuid.UUID
	runningMutex sync.Mutex
}

type JobStatus struct {
	State    common.ComposeState
	Queued   time.Time
	Started  time.Time
	Finished time.Time
	Canceled bool
	Result   OSBuildJobResult
}

var ErrTokenNotExist = errors.New("worker token does not exist")

func NewServer(logger *log.Logger, jobs jobqueue.JobQueue, artifactsDir string) *Server {
	s := &Server{
		jobs:         jobs,
		artifactsDir: artifactsDir,
		running:      make(map[uuid.UUID]uuid.UUID),
	}

	e := echo.New()
	e.Binder = binder{}
	e.StdLogger = logger

	api.RegisterHandlers(e, &apiHandlers{s})

	s.server = &http.Server{
		ErrorLog: logger,
		Handler:  e,
	}

	return s
}

func (s *Server) Serve(listener net.Listener) error {
	err := s.server.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.server.Handler.ServeHTTP(writer, request)
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

func (s *Server) RequestJob(ctx context.Context) (uuid.UUID, uuid.UUID, *OSBuildJob, error) {
	token := uuid.New()

	var args OSBuildJob
	jobId, err := s.jobs.Dequeue(ctx, []string{"osbuild"}, &args)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, err
	}

	if s.artifactsDir != "" {
		err := os.MkdirAll(path.Join(s.artifactsDir, "tmp", token.String()), 0700)
		if err != nil {
			return uuid.Nil, uuid.Nil, nil, fmt.Errorf("cannot create artifact directory: %v", err)
		}
	}

	s.runningMutex.Lock()
	defer s.runningMutex.Unlock()
	s.running[token] = jobId

	return token, jobId, &args, nil
}

func (s *Server) RunningJob(token uuid.UUID) (uuid.UUID, error) {
	s.runningMutex.Lock()
	defer s.runningMutex.Unlock()

	jobId, ok := s.running[token]
	if !ok {
		return uuid.Nil, ErrTokenNotExist
	}

	return jobId, nil
}

func (s *Server) FinishJob(token uuid.UUID, result *OSBuildJobResult) error {
	s.runningMutex.Lock()
	defer s.runningMutex.Unlock()

	jobId, ok := s.running[token]
	if !ok {
		return ErrTokenNotExist
	}

	// Always delete the running job, even if there are errors finishing
	// the job, because callers won't call this a second time on error.
	delete(s.running, token)

	err := s.jobs.FinishJob(jobId, result)
	if err != nil {
		return fmt.Errorf("error finishing job: %v", err)
	}

	// Move artifacts from the temporary location to the final job
	// location. Log any errors, but do not treat them as fatal. The job is
	// already finished.
	if s.artifactsDir != "" {
		err := os.Rename(path.Join(s.artifactsDir, "tmp", token.String()), path.Join(s.artifactsDir, jobId.String()))
		if err != nil {
			log.Printf("Error moving artifacts for job%s: %v", jobId, err)
		}
	}

	return nil
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

func (h *apiHandlers) RequestJob(ctx echo.Context) error {
	var body struct{}
	err := ctx.Bind(&body)
	if err != nil {
		return err
	}

	token, jobId, jobArgs, err := h.server.RequestJob(ctx.Request().Context())
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusCreated, requestJobResponse{
		Id:               jobId,
		Manifest:         jobArgs.Manifest,
		Targets:          jobArgs.Targets,
		Location:         fmt.Sprintf("/jobs/%v", token),
		ArtifactLocation: fmt.Sprintf("/jobs/%v/artifacts/", token),
	})
}

func (h *apiHandlers) GetJob(ctx echo.Context, tokenstr string) error {
	token, err := uuid.Parse(tokenstr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse job token")
	}

	jobId, err := h.server.RunningJob(token)
	if err != nil {
		switch err {
		case ErrTokenNotExist:
			return echo.NewHTTPError(http.StatusNotFound, "not found")
		default:
			return err
		}
	}

	if jobId == uuid.Nil {
		return ctx.JSON(http.StatusOK, getJobResponse{})
	}

	status, err := h.server.JobStatus(jobId)
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK, getJobResponse{
		Canceled: status.Canceled,
	})
}

func (h *apiHandlers) UpdateJob(ctx echo.Context, idstr string) error {
	token, err := uuid.Parse(idstr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse job token")
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

	err = h.server.FinishJob(token, &OSBuildJobResult{OSBuildOutput: body.Result})
	if err != nil {
		switch err {
		case ErrTokenNotExist:
			return echo.NewHTTPError(http.StatusNotFound, "not found")
		default:
			return err
		}
	}

	return ctx.JSON(http.StatusOK, updateJobResponse{})
}

func (h *apiHandlers) UploadJobArtifact(ctx echo.Context, tokenstr string, name string) error {
	token, err := uuid.Parse(tokenstr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse job token")
	}

	request := ctx.Request()

	if h.server.artifactsDir == "" {
		_, err := io.Copy(ioutil.Discard, request.Body)
		if err != nil {
			return fmt.Errorf("error discarding artifact: %v", err)
		}
		return ctx.NoContent(http.StatusOK)
	}

	f, err := os.Create(path.Join(h.server.artifactsDir, "tmp", token.String(), name))
	if err != nil {
		return fmt.Errorf("cannot create artifact file: %v", err)
	}

	_, err = io.Copy(f, request.Body)
	if err != nil {
		return fmt.Errorf("error writing artifact file: %v", err)
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
		return echo.NewHTTPError(http.StatusBadRequest, "cannot parse request body: "+err.Error())
	}

	return nil
}
