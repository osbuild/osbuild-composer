package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue"

	"github.com/osbuild/osbuild-composer/internal/auth"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/prometheus"
	"github.com/osbuild/osbuild-composer/internal/worker/api"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

const (
	JobTypeOSBuild          string = "osbuild"
	JobTypeKojiInit         string = "koji-init"
	JobTypeKojiFinalize     string = "koji-finalize"
	JobTypeDepsolve         string = "depsolve"
	JobTypeManifestIDOnly   string = "manifest-id-only"
	JobTypeContainerResolve string = "container-resolve"
)

type Server struct {
	jobs   jobqueue.JobQueue
	logger *log.Logger
	config Config
}

type JobStatus struct {
	Queued   time.Time
	Started  time.Time
	Finished time.Time
	Canceled bool
}

type JobInfo struct {
	JobType   string
	Channel   string
	JobStatus *JobStatus
	Deps      []uuid.UUID
}

var ErrInvalidToken = errors.New("token does not exist")
var ErrJobNotRunning = errors.New("job isn't running")
var ErrInvalidJobType = errors.New("job has invalid type")

type Config struct {
	ArtifactsDir         string
	RequestJobTimeout    time.Duration
	BasePath             string
	JWTEnabled           bool
	TenantProviderFields []string
}

func NewServer(logger *log.Logger, jobs jobqueue.JobQueue, config Config) *Server {
	s := &Server{
		jobs:   jobs,
		logger: logger,
		config: config,
	}

	api.BasePath = config.BasePath

	go s.WatchHeartbeats()
	return s
}

func (s *Server) Handler() http.Handler {
	e := echo.New()
	e.Binder = binder{}
	e.Logger = common.Logger()

	// log errors returned from handlers
	e.HTTPErrorHandler = api.HTTPErrorHandler
	e.Use(middleware.Recover())
	e.Pre(common.OperationIDMiddleware)
	handler := apiHandlers{
		server: s,
	}
	api.RegisterHandlers(e.Group(api.BasePath), &handler)

	return e
}

// This function should be started as a goroutine
// Every 30 seconds it goes through all running jobs, removing any unresponsive ones.
// It fails jobs which fail to check if they cancelled for more than 2 minutes.
func (s *Server) WatchHeartbeats() {
	//nolint:staticcheck // avoid SA1015, this is an endless function
	for range time.Tick(time.Second * 30) {
		for _, token := range s.jobs.Heartbeats(time.Second * 120) {
			id, _ := s.jobs.IdFromToken(token)
			logrus.Infof("Removing unresponsive job: %s\n", id)

			missingHeartbeatResult := JobResult{
				JobError: clienterrors.WorkerClientError(clienterrors.ErrorJobMissingHeartbeat, "Worker running this job stopped responding."),
			}

			resJson, err := json.Marshal(missingHeartbeatResult)
			if err != nil {
				logrus.Panicf("Cannot marshal the heartbeat error: %v", err)
			}

			err = s.FinishJob(token, resJson)
			if err != nil {
				logrus.Errorf("Error finishing unresponsive job: %v", err)
			}
		}
	}
}

func (s *Server) EnqueueOSBuild(arch string, job *OSBuildJob, channel string) (uuid.UUID, error) {
	return s.enqueue(JobTypeOSBuild+":"+arch, job, nil, channel)
}

func (s *Server) EnqueueOSBuildAsDependency(arch string, job *OSBuildJob, dependencies []uuid.UUID, channel string) (uuid.UUID, error) {
	return s.enqueue(JobTypeOSBuild+":"+arch, job, dependencies, channel)
}

func (s *Server) EnqueueKojiInit(job *KojiInitJob, channel string) (uuid.UUID, error) {
	return s.enqueue(JobTypeKojiInit, job, nil, channel)
}

func (s *Server) EnqueueKojiFinalize(job *KojiFinalizeJob, initID uuid.UUID, buildIDs []uuid.UUID, channel string) (uuid.UUID, error) {
	return s.enqueue(JobTypeKojiFinalize, job, append([]uuid.UUID{initID}, buildIDs...), channel)
}

func (s *Server) EnqueueDepsolve(job *DepsolveJob, channel string) (uuid.UUID, error) {
	return s.enqueue(JobTypeDepsolve, job, nil, channel)
}

func (s *Server) EnqueueManifestJobByID(job *ManifestJobByID, parent uuid.UUID, channel string) (uuid.UUID, error) {
	return s.enqueue(JobTypeManifestIDOnly, job, []uuid.UUID{parent}, channel)
}

func (s *Server) EnqueueContainerResolveJob(job *ContainerResolveJob, channel string) (uuid.UUID, error) {
	return s.enqueue(JobTypeContainerResolve, job, nil, channel)
}

func (s *Server) enqueue(jobType string, job interface{}, dependencies []uuid.UUID, channel string) (uuid.UUID, error) {
	prometheus.EnqueueJobMetrics(strings.Split(jobType, ":")[0], channel)
	return s.jobs.Enqueue(jobType, job, dependencies, channel)
}

// DependencyChainErrors recursively gathers all errors from job's dependencies,
// which caused it to fail. If the job didn't fail, `nil` is returned.
func (s *Server) JobDependencyChainErrors(id uuid.UUID) (*clienterrors.Error, error) {
	jobType, err := s.JobType(id)
	if err != nil {
		return nil, err
	}

	var jobResult *JobResult
	var jobInfo *JobInfo
	switch jobType {
	case JobTypeOSBuild:
		var osbuildJR OSBuildJobResult
		jobInfo, err = s.OSBuildJobInfo(id, &osbuildJR)
		if err != nil {
			return nil, err
		}
		jobResult = &osbuildJR.JobResult

	case JobTypeDepsolve:
		var depsolveJR DepsolveJobResult
		jobInfo, err = s.DepsolveJobInfo(id, &depsolveJR)
		if err != nil {
			return nil, err
		}
		jobResult = &depsolveJR.JobResult

	case JobTypeManifestIDOnly:
		var manifestJR ManifestJobByIDResult
		jobInfo, err = s.ManifestJobInfo(id, &manifestJR)
		if err != nil {
			return nil, err
		}
		jobResult = &manifestJR.JobResult

	case JobTypeKojiInit:
		var kojiInitJR KojiInitJobResult
		jobInfo, err = s.KojiInitJobInfo(id, &kojiInitJR)
		if err != nil {
			return nil, err
		}
		jobResult = &kojiInitJR.JobResult

	case JobTypeKojiFinalize:
		var kojiFinalizeJR KojiFinalizeJobResult
		jobInfo, err = s.KojiFinalizeJobInfo(id, &kojiFinalizeJR)
		if err != nil {
			return nil, err
		}
		jobResult = &kojiFinalizeJR.JobResult

	case JobTypeContainerResolve:
		var containerResolveJR ContainerResolveJobResult
		jobInfo, err = s.ContainerResolveJobInfo(id, &containerResolveJR)
		if err != nil {
			return nil, err
		}
		jobResult = &containerResolveJR.JobResult

	default:
		return nil, fmt.Errorf("unexpected job type: %s", jobType)
	}

	if jobError := jobResult.JobError; jobError != nil {
		depErrors := []*clienterrors.Error{}
		if jobError.IsDependencyError() {
			// check job's dependencies
			for _, dep := range jobInfo.Deps {
				depError, err := s.JobDependencyChainErrors(dep)
				if err != nil {
					return nil, err
				}
				if depError != nil {
					depErrors = append(depErrors, depError)
				}
			}
		}

		if len(depErrors) > 0 {
			jobError.Details = depErrors
		}

		return jobError, nil
	}

	return nil, nil
}

func (s *Server) OSBuildJobInfo(id uuid.UUID, result *OSBuildJobResult) (*JobInfo, error) {
	jobInfo, err := s.jobInfo(id, result)
	if err != nil {
		return nil, err
	}

	if jobInfo.JobType != JobTypeOSBuild {
		return nil, fmt.Errorf("expected %q, found %q job instead", JobTypeOSBuild, jobInfo.JobType)
	}

	if result.JobError == nil && !jobInfo.JobStatus.Finished.IsZero() {
		if result.OSBuildOutput == nil {
			result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorBuildJob, "osbuild build failed")
		} else if len(result.OSBuildOutput.Error) > 0 {
			result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorOldResultCompatible, string(result.OSBuildOutput.Error))
		} else if len(result.TargetErrors()) > 0 {
			result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorTargetError, "at least one target failed", result.TargetErrors())
		}
	}
	// For backwards compatibility: OSBuildJobResult didn't use to have a
	// top-level `Success` flag. Override it here by looking into the job.
	if !result.Success && result.OSBuildOutput != nil {
		result.Success = result.OSBuildOutput.Success && result.JobError == nil
	}

	return jobInfo, nil
}

func (s *Server) KojiInitJobInfo(id uuid.UUID, result *KojiInitJobResult) (*JobInfo, error) {
	jobInfo, err := s.jobInfo(id, result)
	if err != nil {
		return nil, err
	}

	if jobInfo.JobType != JobTypeKojiInit {
		return nil, fmt.Errorf("expected %q, found %q job instead", JobTypeKojiInit, jobInfo.JobType)
	}

	if result.JobError == nil && result.KojiError != "" {
		result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorOldResultCompatible, result.KojiError)
	}

	return jobInfo, nil
}

func (s *Server) KojiFinalizeJobInfo(id uuid.UUID, result *KojiFinalizeJobResult) (*JobInfo, error) {
	jobInfo, err := s.jobInfo(id, result)
	if err != nil {
		return nil, err
	}

	if jobInfo.JobType != JobTypeKojiFinalize {
		return nil, fmt.Errorf("expected %q, found %q job instead", JobTypeKojiFinalize, jobInfo.JobType)
	}

	if result.JobError == nil && result.KojiError != "" {
		result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorOldResultCompatible, result.KojiError)
	}

	return jobInfo, nil
}

func (s *Server) DepsolveJobInfo(id uuid.UUID, result *DepsolveJobResult) (*JobInfo, error) {
	jobInfo, err := s.jobInfo(id, result)
	if err != nil {
		return nil, err
	}

	if jobInfo.JobType != JobTypeDepsolve {
		return nil, fmt.Errorf("expected %q, found %q job instead", JobTypeDepsolve, jobInfo.JobType)
	}

	if result.JobError == nil && result.Error != "" {
		if result.ErrorType == DepsolveErrorType {
			result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDNFDepsolveError, result.Error)
		} else {
			result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorRPMMDError, result.Error)
		}
	}

	return jobInfo, nil
}

func (s *Server) ManifestJobInfo(id uuid.UUID, result *ManifestJobByIDResult) (*JobInfo, error) {
	jobInfo, err := s.jobInfo(id, result)
	if err != nil {
		return nil, err
	}

	if jobInfo.JobType != JobTypeManifestIDOnly {
		return nil, fmt.Errorf("expected %q, found %q job instead", JobTypeManifestIDOnly, jobInfo.JobType)
	}

	return jobInfo, nil
}

func (s *Server) ContainerResolveJobInfo(id uuid.UUID, result *ContainerResolveJobResult) (*JobInfo, error) {
	jobInfo, err := s.jobInfo(id, result)

	if err != nil {
		return nil, err
	}

	if jobInfo.JobType != JobTypeContainerResolve {
		return nil, fmt.Errorf("expected %q, found %q job instead", JobTypeDepsolve, jobInfo.JobType)
	}

	return jobInfo, nil
}

func (s *Server) jobInfo(id uuid.UUID, result interface{}) (*JobInfo, error) {
	jobType, channel, rawResult, queued, started, finished, canceled, deps, err := s.jobs.JobStatus(id)
	if err != nil {
		return nil, err
	}

	if result != nil && !finished.IsZero() && !canceled {
		err = json.Unmarshal(rawResult, result)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling result for job '%s': %v", id, err)
		}
	}

	return &JobInfo{
		JobType: strings.Split(jobType, ":")[0],
		Channel: channel,
		JobStatus: &JobStatus{
			Queued:   queued,
			Started:  started,
			Finished: finished,
			Canceled: canceled,
		},
		Deps: deps,
	}, nil
}

// OSBuildJob returns the parameters of an OSBuildJob
func (s *Server) OSBuildJob(id uuid.UUID, job *OSBuildJob) error {
	jobType, rawArgs, _, _, err := s.jobs.Job(id)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(jobType, JobTypeOSBuild+":") { // Build jobs get automatic arch suffix: Check prefix
		return fmt.Errorf("expected %s:*, found %q job instead for job '%s'", JobTypeOSBuild, jobType, id)
	}

	if err := json.Unmarshal(rawArgs, job); err != nil {
		return fmt.Errorf("error unmarshaling arguments for job '%s': %v", id, err)
	}

	return nil
}

func (s *Server) JobChannel(id uuid.UUID) (string, error) {
	_, _, _, channel, err := s.jobs.Job(id)
	return channel, err
}

// JobType returns the type of the job
func (s *Server) JobType(id uuid.UUID) (string, error) {
	jobType, _, _, _, err := s.jobs.Job(id)
	// the architecture is internally encdode in the job type, but hide that
	// from this API
	return strings.Split(jobType, ":")[0], err
}

func (s *Server) Cancel(id uuid.UUID) error {
	jobInfo, err := s.jobInfo(id, nil)
	if err != nil {
		logrus.Errorf("error getting job status: %v", err)
	} else {
		prometheus.CancelJobMetrics(jobInfo.JobStatus.Started, jobInfo.JobType, jobInfo.Channel)
	}
	return s.jobs.CancelJob(id)
}

// Provides access to artifacts of a job. Returns an io.Reader for the artifact
// and the artifact's size.
func (s *Server) JobArtifact(id uuid.UUID, name string) (io.Reader, int64, error) {
	if s.config.ArtifactsDir == "" {
		return nil, 0, errors.New("Artifacts not enabled")
	}

	jobInfo, err := s.jobInfo(id, nil)
	if err != nil {
		return nil, 0, err
	}

	if jobInfo.JobStatus.Finished.IsZero() {
		return nil, 0, fmt.Errorf("Cannot access artifacts before job is finished: %s", id)
	}

	p := path.Join(s.config.ArtifactsDir, id.String(), name)
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
	if s.config.ArtifactsDir == "" {
		return errors.New("Artifacts not enabled")
	}

	jobInfo, err := s.jobInfo(id, nil)
	if err != nil {
		return err
	}

	if jobInfo.JobStatus.Finished.IsZero() {
		return fmt.Errorf("Cannot delete artifacts before job is finished: %s", id)
	}

	return os.RemoveAll(path.Join(s.config.ArtifactsDir, id.String()))
}

func (s *Server) RequestJob(ctx context.Context, arch string, jobTypes []string, channels []string) (uuid.UUID, uuid.UUID, string, json.RawMessage, []json.RawMessage, error) {
	return s.requestJob(ctx, arch, jobTypes, uuid.Nil, channels)
}

func (s *Server) RequestJobById(ctx context.Context, arch string, requestedJobId uuid.UUID) (uuid.UUID, uuid.UUID, string, json.RawMessage, []json.RawMessage, error) {
	return s.requestJob(ctx, arch, []string{}, requestedJobId, nil)
}

func (s *Server) requestJob(ctx context.Context, arch string, jobTypes []string, requestedJobId uuid.UUID, channels []string) (
	jobId uuid.UUID, token uuid.UUID, jobType string, args json.RawMessage, dynamicArgs []json.RawMessage, err error) {
	// treat osbuild jobs specially until we have found a generic way to
	// specify dequeuing restrictions. For now, we only have one
	// restriction: arch for osbuild jobs.
	jts := []string{}
	for _, t := range jobTypes {
		if t == JobTypeOSBuild {
			t = t + ":" + arch
		}
		if t == JobTypeManifestIDOnly {
			return uuid.Nil, uuid.Nil, "", nil, nil, ErrInvalidJobType
		}
		jts = append(jts, t)
	}

	dequeueCtx := ctx
	var cancel context.CancelFunc
	if s.config.RequestJobTimeout != 0 {
		dequeueCtx, cancel = context.WithTimeout(ctx, s.config.RequestJobTimeout)
		defer cancel()
	}

	var depIDs []uuid.UUID
	if requestedJobId != uuid.Nil {
		jobId = requestedJobId
		token, depIDs, jobType, args, err = s.jobs.DequeueByID(dequeueCtx, requestedJobId)
	} else {
		jobId, token, depIDs, jobType, args, err = s.jobs.Dequeue(dequeueCtx, jts, channels)
	}
	if err != nil {
		return
	}

	jobInfo, err := s.jobInfo(jobId, nil)
	if err != nil {
		logrus.Errorf("error retrieving job status: %v", err)
	}

	// Record how long the job has been pending for, that is either how
	// long it has been queued for, in case it has no dependencies, or
	// how long it has been since all its dependencies finished, if it
	// has any.
	pending := jobInfo.JobStatus.Queued
	jobType = jobInfo.JobType

	for _, depID := range depIDs {
		// TODO: include type of arguments
		var result json.RawMessage
		var finished time.Time
		_, _, result, _, _, finished, _, _, err = s.jobs.JobStatus(depID)
		if err != nil {
			return
		}
		if finished.After(pending) {
			pending = finished
		}
		dynamicArgs = append(dynamicArgs, result)
	}

	if s.config.ArtifactsDir != "" {
		err = os.MkdirAll(path.Join(s.config.ArtifactsDir, "tmp", token.String()), 0700)
		if err != nil {
			return
		}
	}

	prometheus.DequeueJobMetrics(pending, jobInfo.JobStatus.Started, jobInfo.JobType, jobInfo.Channel)

	return
}

func (s *Server) FinishJob(token uuid.UUID, result json.RawMessage) error {
	jobId, err := s.jobs.IdFromToken(token)
	if err != nil {
		switch err {
		case jobqueue.ErrNotExist:
			return ErrInvalidToken
		default:
			return err
		}
	}

	err = s.jobs.FinishJob(jobId, result)
	if err != nil {
		switch err {
		case jobqueue.ErrNotRunning:
			return ErrJobNotRunning
		default:
			return fmt.Errorf("error finishing job: %v", err)
		}
	}

	jobType, err := s.JobType(jobId)
	if err != nil {
		return err
	}

	var arch string
	var jobInfo *JobInfo
	var jobResult *JobResult
	switch jobType {
	case JobTypeOSBuild:
		var osbuildJR OSBuildJobResult
		jobInfo, err = s.OSBuildJobInfo(jobId, &osbuildJR)
		if err != nil {
			return err
		}
		arch = osbuildJR.Arch
		jobResult = &osbuildJR.JobResult

	case JobTypeDepsolve:
		var depsolveJR DepsolveJobResult
		jobInfo, err = s.DepsolveJobInfo(jobId, &depsolveJR)
		if err != nil {
			return err
		}
		jobResult = &depsolveJR.JobResult

	case JobTypeManifestIDOnly:
		var manifestJR ManifestJobByIDResult
		jobInfo, err = s.ManifestJobInfo(jobId, &manifestJR)
		if err != nil {
			return err
		}
		jobResult = &manifestJR.JobResult

	case JobTypeKojiInit:
		var kojiInitJR KojiInitJobResult
		jobInfo, err = s.KojiInitJobInfo(jobId, &kojiInitJR)
		if err != nil {
			return err
		}
		jobResult = &kojiInitJR.JobResult

	case JobTypeKojiFinalize:
		var kojiFinalizeJR KojiFinalizeJobResult
		jobInfo, err = s.KojiFinalizeJobInfo(jobId, &kojiFinalizeJR)
		if err != nil {
			return err
		}
		jobResult = &kojiFinalizeJR.JobResult

	default:
		return fmt.Errorf("unexpected job type: %s", jobType)
	}

	statusCode := clienterrors.GetStatusCode(jobResult.JobError)
	prometheus.FinishJobMetrics(jobInfo.JobStatus.Started, jobInfo.JobStatus.Finished, jobInfo.JobStatus.Canceled, jobType, jobInfo.Channel, arch, statusCode)

	// Move artifacts from the temporary location to the final job
	// location. Log any errors, but do not treat them as fatal. The job is
	// already finished.
	if s.config.ArtifactsDir != "" {
		err := os.Rename(path.Join(s.config.ArtifactsDir, "tmp", token.String()), path.Join(s.config.ArtifactsDir, jobId.String()))
		if err != nil {
			logrus.Errorf("Error moving artifacts for job %s: %v", jobId, err)
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

func (h *apiHandlers) GetOpenapi(ctx echo.Context) error {
	spec, err := api.GetSwagger()
	if err != nil {
		return api.HTTPErrorWithInternal(api.ErrorFailedLoadingOpenAPISpec, err)
	}
	return ctx.JSON(http.StatusOK, spec)
}

func (h *apiHandlers) GetStatus(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, &api.StatusResponse{
		ObjectReference: api.ObjectReference{
			Href: fmt.Sprintf("%s/status", api.BasePath),
			Id:   "status",
			Kind: "Status",
		},
		Status: "OK",
	})
}

func (h *apiHandlers) GetError(ctx echo.Context, id string) error {
	errorId, err := strconv.Atoi(id)
	if err != nil {
		return api.HTTPErrorWithInternal(api.ErrorInvalidErrorId, err)
	}

	apiError := api.APIError(api.ServiceErrorCode(errorId), nil, ctx)
	// If the service error wasn't found, it's a 404 in this instance
	if apiError.Id == fmt.Sprintf("%d", api.ErrorServiceErrorNotFound) {
		return api.HTTPError(api.ErrorErrorNotFound)
	}
	return ctx.JSON(http.StatusOK, apiError)
}

func (h *apiHandlers) RequestJob(ctx echo.Context) error {
	var body api.RequestJobJSONRequestBody
	err := ctx.Bind(&body)
	if err != nil {
		return err
	}

	// channel is empty if JWT is not enabled
	var channel string
	if h.server.config.JWTEnabled {
		tenant, err := auth.GetFromClaims(ctx.Request().Context(), h.server.config.TenantProviderFields)
		if err != nil {
			return api.HTTPErrorWithInternal(api.ErrorTenantNotFound, err)
		}

		// prefix the tenant to prevent collisions if support for specifying channels in a request is ever added
		channel = "org-" + tenant
	}

	jobId, jobToken, jobType, jobArgs, dynamicJobArgs, err := h.server.RequestJob(ctx.Request().Context(), body.Arch, body.Types, []string{channel})
	if err != nil {
		if err == jobqueue.ErrDequeueTimeout {
			return ctx.JSON(http.StatusNoContent, api.ObjectReference{
				Href: fmt.Sprintf("%s/jobs", api.BasePath),
				Id:   uuid.Nil.String(),
				Kind: "RequestJob",
			})
		}
		if err == ErrInvalidJobType {
			return api.HTTPError(api.ErrorInvalidJobType)
		}
		return api.HTTPErrorWithInternal(api.ErrorRequestingJob, err)
	}

	var respArgs *json.RawMessage
	if len(jobArgs) != 0 {
		respArgs = &jobArgs
	}
	var respDynArgs *[]json.RawMessage
	if len(dynamicJobArgs) != 0 {
		respDynArgs = &dynamicJobArgs
	}

	response := api.RequestJobResponse{
		ObjectReference: api.ObjectReference{
			Href: fmt.Sprintf("%s/jobs", api.BasePath),
			Id:   jobId.String(),
			Kind: "RequestJob",
		},
		Location:         fmt.Sprintf("%s/jobs/%v", api.BasePath, jobToken),
		ArtifactLocation: fmt.Sprintf("%s/jobs/%v/artifacts/", api.BasePath, jobToken),
		Type:             jobType,
		Args:             respArgs,
		DynamicArgs:      respDynArgs,
	}
	return ctx.JSON(http.StatusCreated, response)
}

func (h *apiHandlers) GetJob(ctx echo.Context, tokenstr string) error {
	token, err := uuid.Parse(tokenstr)
	if err != nil {
		return api.HTTPErrorWithInternal(api.ErrorMalformedJobToken, err)
	}

	jobId, err := h.server.jobs.IdFromToken(token)
	if err != nil {
		switch err {
		case jobqueue.ErrNotExist:
			return api.HTTPError(api.ErrorJobNotFound)
		default:
			return api.HTTPErrorWithInternal(api.ErrorResolvingJobId, err)
		}
	}

	if jobId == uuid.Nil {
		return ctx.JSON(http.StatusOK, api.GetJobResponse{
			ObjectReference: api.ObjectReference{
				Href: fmt.Sprintf("%s/jobs/%v", api.BasePath, token),
				Id:   token.String(),
				Kind: "JobStatus",
			},
			Canceled: false,
		})
	}

	h.server.jobs.RefreshHeartbeat(token)

	jobInfo, err := h.server.jobInfo(jobId, nil)
	if err != nil {
		return api.HTTPErrorWithInternal(api.ErrorRetrievingJobStatus, err)
	}

	return ctx.JSON(http.StatusOK, api.GetJobResponse{
		ObjectReference: api.ObjectReference{
			Href: fmt.Sprintf("%s/jobs/%v", api.BasePath, token),
			Id:   token.String(),
			Kind: "JobStatus",
		},
		Canceled: jobInfo.JobStatus.Canceled,
	})
}

func (h *apiHandlers) UpdateJob(ctx echo.Context, idstr string) error {
	token, err := uuid.Parse(idstr)
	if err != nil {
		return api.HTTPErrorWithInternal(api.ErrorMalformedJobId, err)
	}

	var body api.UpdateJobRequest
	err = ctx.Bind(&body)
	if err != nil {
		return err
	}

	err = h.server.FinishJob(token, body.Result)
	if err != nil {
		switch err {
		case ErrInvalidToken:
			return api.HTTPError(api.ErrorJobNotFound)
		case ErrJobNotRunning:
			return api.HTTPError(api.ErrorJobNotRunning)
		default:
			return api.HTTPErrorWithInternal(api.ErrorFinishingJob, err)
		}
	}

	return ctx.JSON(http.StatusOK, api.UpdateJobResponse{
		Href: fmt.Sprintf("%s/jobs/%v", api.BasePath, token),
		Id:   token.String(),
		Kind: "UpdateJobResponse",
	})
}

func (h *apiHandlers) UploadJobArtifact(ctx echo.Context, tokenstr string, name string) error {
	token, err := uuid.Parse(tokenstr)
	if err != nil {
		return api.HTTPErrorWithInternal(api.ErrorMalformedJobId, err)
	}

	request := ctx.Request()

	if h.server.config.ArtifactsDir == "" {
		// indicate to the worker that the server is not accepting any artifacts
		return ctx.NoContent(http.StatusBadRequest)
	}

	f, err := os.Create(path.Join(h.server.config.ArtifactsDir, "tmp", token.String(), name))
	if err != nil {
		return api.HTTPErrorWithInternal(api.ErrorDiscardingArtifact, err)
	}

	_, err = io.Copy(f, request.Body)
	if err != nil {
		return api.HTTPErrorWithInternal(api.ErrorWritingArtifact, err)
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
		return api.HTTPError(api.ErrorUnsupportedMediaType)
	}

	err := json.NewDecoder(request.Body).Decode(i)
	if err != nil {
		return api.HTTPErrorWithInternal(api.ErrorBodyDecodingError, err)
	}

	return nil
}
