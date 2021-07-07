package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/prometheus"
	"github.com/osbuild/osbuild-composer/internal/worker/api"
)

type Server struct {
	jobs           jobqueue.JobQueue
	logger         *log.Logger
	artifactsDir   string
	identityFilter []string

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
	Queued   time.Time
	Started  time.Time
	Finished time.Time
	Canceled bool
}

var ErrTokenNotExist = errors.New("worker token does not exist")

func NewServer(logger *log.Logger, jobs jobqueue.JobQueue, artifactsDir string, identityFilter []string) *Server {

	return &Server{
		jobs:           jobs,
		logger:         logger,
		artifactsDir:   artifactsDir,
		identityFilter: identityFilter,
		running:        make(map[uuid.UUID]uuid.UUID),
	}
}

func (s *Server) Handler() http.Handler {
	e := echo.New()
	e.Binder = binder{}
	e.StdLogger = s.logger

	// log errors returned from handlers
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		log.Println(c.Path(), c.QueryParams().Encode(), err.Error())
		e.DefaultHTTPErrorHandler(err, c)
	}

	var mws []echo.MiddlewareFunc
	if len(s.identityFilter) > 0 {
		mws = append(mws, s.VerifyIdentityHeader)
	}

	handler := apiHandlers{
		server: s,
	}
	api.RegisterHandlers(e.Group(api.BasePath, mws...), &handler)
	api.RegisterHandlers(e.Group(api.CloudBasePath, mws...), &handler)

	return e
}

func (s *Server) VerifyIdentityHeader(nextHandler echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		type identityHeader struct {
			Identity struct {
				AccountNumber string `json:"account_number"`
			} `json:"identity"`
		}

		request := ctx.Request()

		idHeaderB64 := request.Header["X-Rh-Identity"]
		if len(idHeaderB64) != 1 {
			return echo.NewHTTPError(http.StatusNotFound, "Auth header is not present")
		}

		b64Result, err := base64.StdEncoding.DecodeString(idHeaderB64[0])
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "Auth header has incorrect format")
		}

		var idHeader identityHeader
		err = json.Unmarshal([]byte(strings.TrimSuffix(fmt.Sprintf("%s", b64Result), "\n")), &idHeader)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "Auth header has incorrect format")
		}

		for _, i := range s.identityFilter {
			if idHeader.Identity.AccountNumber == i {
				ctx.Set("IdentityHeader", idHeader)
				return nextHandler(ctx)

			}
		}
		return echo.NewHTTPError(http.StatusNotFound, "Account not allowed")
	}
}

func (s *Server) EnqueueOSBuild(arch string, job *OSBuildJob) (uuid.UUID, error) {
	return s.jobs.Enqueue("osbuild:"+arch, job, nil)
}

func (s *Server) EnqueueOSBuildKoji(arch string, job *OSBuildKojiJob, initID uuid.UUID) (uuid.UUID, error) {
	return s.jobs.Enqueue("osbuild-koji:"+arch, job, []uuid.UUID{initID})
}

func (s *Server) EnqueueKojiInit(job *KojiInitJob) (uuid.UUID, error) {
	return s.jobs.Enqueue("koji-init", job, nil)
}

func (s *Server) EnqueueKojiFinalize(job *KojiFinalizeJob, initID uuid.UUID, buildIDs []uuid.UUID) (uuid.UUID, error) {
	return s.jobs.Enqueue("koji-finalize", job, append([]uuid.UUID{initID}, buildIDs...))
}

func (s *Server) JobStatus(id uuid.UUID, result interface{}) (*JobStatus, []uuid.UUID, error) {
	rawResult, queued, started, finished, canceled, deps, err := s.jobs.JobStatus(id)
	if err != nil {
		return nil, nil, err
	}

	if !finished.IsZero() && !canceled {
		err = json.Unmarshal(rawResult, result)
		if err != nil {
			return nil, nil, fmt.Errorf("error unmarshaling result for job '%s': %v", id, err)
		}
	}

	// For backwards compatibility: OSBuildJobResult didn't use to have a
	// top-level `Success` flag. Override it here by looking into the job.
	if r, ok := result.(*OSBuildJobResult); ok {
		if !r.Success && r.OSBuildOutput != nil {
			r.Success = r.OSBuildOutput.Success && len(r.TargetErrors) == 0
		}
	}

	return &JobStatus{
		Queued:   queued,
		Started:  started,
		Finished: finished,
		Canceled: canceled,
	}, deps, nil
}

// Job provides access to all the parameters of a job.
func (s *Server) Job(id uuid.UUID, job interface{}) (string, json.RawMessage, []uuid.UUID, error) {
	jobType, rawArgs, deps, err := s.jobs.Job(id)
	if err != nil {
		return "", nil, nil, err
	}

	if err := json.Unmarshal(rawArgs, job); err != nil {
		return "", nil, nil, fmt.Errorf("error unmarshaling arguments for job '%s': %v", id, err)
	}

	return jobType, rawArgs, deps, nil
}

func (s *Server) Cancel(id uuid.UUID) error {
	return s.jobs.CancelJob(id)
}

// Provides access to artifacts of a job. Returns an io.Reader for the artifact
// and the artifact's size.
func (s *Server) JobArtifact(id uuid.UUID, name string) (io.Reader, int64, error) {
	if s.artifactsDir == "" {
		return nil, 0, errors.New("Artifacts not enabled")
	}

	status, _, err := s.JobStatus(id, &json.RawMessage{})
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
	if s.artifactsDir == "" {
		return errors.New("Artifacts not enabled")
	}

	status, _, err := s.JobStatus(id, &json.RawMessage{})
	if err != nil {
		return err
	}

	if status.Finished.IsZero() {
		return fmt.Errorf("Cannot delete artifacts before job is finished: %s", id)
	}

	return os.RemoveAll(path.Join(s.artifactsDir, id.String()))
}

func (s *Server) RequestJob(ctx context.Context, arch string, jobTypes []string) (uuid.UUID, uuid.UUID, string, json.RawMessage, []json.RawMessage, error) {
	token := uuid.New()

	// treat osbuild jobs specially until we have found a generic way to
	// specify dequeuing restrictions. For now, we only have one
	// restriction: arch for osbuild jobs.
	jts := []string{}
	for _, t := range jobTypes {
		if t == "osbuild" || t == "osbuild-koji" {
			t = t + ":" + arch
		}
		jts = append(jts, t)
	}

	jobId, depIDs, jobType, args, err := s.jobs.Dequeue(ctx, jts)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", nil, nil, err
	}

	var dynamicArgs []json.RawMessage
	for _, depID := range depIDs {
		result, _, _, _, _, _, _ := s.jobs.JobStatus(depID)
		dynamicArgs = append(dynamicArgs, result)
	}

	if s.artifactsDir != "" {
		err := os.MkdirAll(path.Join(s.artifactsDir, "tmp", token.String()), 0700)
		if err != nil {
			return uuid.Nil, uuid.Nil, "", nil, nil, fmt.Errorf("cannot create artifact directory: %v", err)
		}
	}

	s.runningMutex.Lock()
	defer s.runningMutex.Unlock()
	s.running[token] = jobId

	if jobType == "osbuild:"+arch {
		jobType = "osbuild"
	} else if jobType == "osbuild-koji:"+arch {
		jobType = "osbuild-koji"
	}

	return token, jobId, jobType, args, dynamicArgs, nil
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

func (s *Server) FinishJob(token uuid.UUID, result json.RawMessage) error {
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

	var jobResult OSBuildJobResult
	_, _, err = s.JobStatus(jobId, &jobResult)
	if err != nil {
		return fmt.Errorf("error finding job status: %v", err)
	}

	if jobResult.Success {
		prometheus.ComposeSuccesses.Inc()
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
	var body api.RequestJobJSONRequestBody
	err := ctx.Bind(&body)
	if err != nil {
		return err
	}

	token, jobId, jobType, jobArgs, dynamicJobArgs, err := h.server.RequestJob(ctx.Request().Context(), body.Arch, body.Types)
	if err != nil {
		return err
	}

	basePath := api.BasePath
	if strings.HasPrefix(ctx.Path(), api.CloudBasePath) {
		basePath = api.CloudBasePath
	}

	return ctx.JSON(http.StatusCreated, requestJobResponse{
		Id:               jobId,
		Location:         fmt.Sprintf("%s/jobs/%v", basePath, token),
		ArtifactLocation: fmt.Sprintf("%s/jobs/%v/artifacts/", basePath, token),
		Type:             jobType,
		Args:             jobArgs,
		DynamicArgs:      dynamicJobArgs,
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

	status, _, err := h.server.JobStatus(jobId, &json.RawMessage{})
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

	err = h.server.FinishJob(token, body.Result)
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
