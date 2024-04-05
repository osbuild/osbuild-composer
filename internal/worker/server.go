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
	JobTypeFileResolve      string = "file-resolve"
	JobTypeOSTreeResolve    string = "ostree-resolve"
	JobTypeAWSEC2Copy       string = "aws-ec2-copy"
	JobTypeAWSEC2Share      string = "aws-ec2-share"
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
	JobType    string
	Channel    string
	JobStatus  *JobStatus
	Deps       []uuid.UUID
	Dependents []uuid.UUID
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
	JobTimeout           time.Duration
	JobWatchFreq         time.Duration
	WorkerTimeout        time.Duration
	WorkerWatchFreq      time.Duration
}

func NewServer(logger *log.Logger, jobs jobqueue.JobQueue, config Config) *Server {
	s := &Server{
		jobs:   jobs,
		logger: logger,
		config: config,
	}

	if s.config.JobTimeout == 0 {
		s.config.JobTimeout = time.Second * 120
	}
	if s.config.JobWatchFreq == 0 {
		s.config.JobWatchFreq = time.Second * 30
	}
	if s.config.WorkerTimeout == 0 {
		s.config.WorkerTimeout = time.Hour
	}
	if s.config.WorkerWatchFreq == 0 {
		s.config.WorkerWatchFreq = time.Second * 300
	}

	api.BasePath = config.BasePath

	go s.WatchHeartbeats()
	go s.WatchWorkers()
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

	mws := []echo.MiddlewareFunc{
		prometheus.StatusMiddleware(prometheus.WorkerSubsystem),
	}
	if s.config.JWTEnabled {
		mws = append(mws, auth.TenantChannelMiddleware(s.config.TenantProviderFields, api.HTTPError(api.ErrorTenantNotFound)))
	}
	mws = append(mws, prometheus.HTTPDurationMiddleware(prometheus.WorkerSubsystem))
	api.RegisterHandlers(e.Group(api.BasePath, mws...), &handler)

	return e
}

const maxHeartbeatRetries = 2

// This function should be started as a goroutine

// With default durations it goes through all running jobs every 30 seconds and fails any unresponsive
// ones. Unresponsive jobs haven't checked whether or not they're cancelled in the past 2 minutes.
func (s *Server) WatchHeartbeats() {
	//nolint:staticcheck // avoid SA1015, this is an endless function
	for range time.Tick(s.config.JobWatchFreq) {
		for _, token := range s.jobs.Heartbeats(s.config.JobTimeout) {
			id, _ := s.jobs.IdFromToken(token)
			logrus.Infof("Removing unresponsive job: %s\n", id)

			missingHeartbeatResult := JobResult{
				JobError: clienterrors.New(clienterrors.ErrorJobMissingHeartbeat,
					fmt.Sprintf("Workers running this job stopped responding more than %d times.", maxHeartbeatRetries),
					nil),
			}

			resJson, err := json.Marshal(missingHeartbeatResult)
			if err != nil {
				logrus.Panicf("Cannot marshal the heartbeat error: %v", err)
			}

			err = s.RequeueOrFinishJob(token, maxHeartbeatRetries, resJson)
			if err != nil {
				logrus.Errorf("Error requeueing or finishing unresponsive job: %v", err)
			}
		}
	}
}

// This function should be started as a goroutine
// Every 5 minutes it goes through all workers, removing any unresponsive ones.
func (s *Server) WatchWorkers() {
	//nolint:staticcheck // avoid SA1015, this is an endless function
	for range time.Tick(s.config.WorkerWatchFreq) {
		workers, err := s.jobs.Workers(s.config.WorkerTimeout)
		if err != nil {
			logrus.Warningf("Unable to query workers: %v", err)
			continue
		}
		for _, w := range workers {
			logrus.Infof("Removing inactive worker: %s", w.ID)
			err = s.jobs.DeleteWorker(w.ID)
			if err != nil {
				logrus.Warningf("Unable to remove worker: %v", err)
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

func (s *Server) EnqueueManifestJobByID(job *ManifestJobByID, dependencies []uuid.UUID, channel string) (uuid.UUID, error) {
	if len(dependencies) == 0 {
		panic("EnqueueManifestJobByID has no dependencies, expected at least a depsolve job")
	}
	return s.enqueue(JobTypeManifestIDOnly, job, dependencies, channel)
}

func (s *Server) EnqueueContainerResolveJob(job *ContainerResolveJob, channel string) (uuid.UUID, error) {
	return s.enqueue(JobTypeContainerResolve, job, nil, channel)
}

func (s *Server) EnqueueFileResolveJob(job *FileResolveJob, channel string) (uuid.UUID, error) {
	return s.enqueue(JobTypeFileResolve, job, nil, channel)
}

func (s *Server) EnqueueOSTreeResolveJob(job *OSTreeResolveJob, channel string) (uuid.UUID, error) {
	return s.enqueue(JobTypeOSTreeResolve, job, nil, channel)
}

func (s *Server) EnqueueAWSEC2CopyJob(job *AWSEC2CopyJob, parent uuid.UUID, channel string) (uuid.UUID, error) {
	return s.enqueue(JobTypeAWSEC2Copy, job, []uuid.UUID{parent}, channel)
}

func (s *Server) EnqueueAWSEC2ShareJob(job *AWSEC2ShareJob, parent uuid.UUID, channel string) (uuid.UUID, error) {
	return s.enqueue(JobTypeAWSEC2Share, job, []uuid.UUID{parent}, channel)
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
	case JobTypeFileResolve:
		var fileResolveJR FileResolveJobResult
		jobInfo, err = s.FileResolveJobInfo(id, &fileResolveJR)
		if err != nil {
			return nil, err
		}
		jobResult = &fileResolveJR.JobResult
	case JobTypeOSTreeResolve:
		var ostreeResolveJR OSTreeResolveJobResult
		jobInfo, err = s.OSTreeResolveJobInfo(id, &ostreeResolveJR)
		if err != nil {
			return nil, err
		}
		jobResult = &ostreeResolveJR.JobResult

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

// AllRootJobIDs returns a list of top level job UUIDs that the worker knows about
func (s *Server) AllRootJobIDs() ([]uuid.UUID, error) {
	return s.jobs.AllRootJobIDs()
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
			result.JobError = clienterrors.New(clienterrors.ErrorBuildJob, "osbuild build failed", nil)
		} else if len(result.OSBuildOutput.Error) > 0 {
			result.JobError = clienterrors.New(clienterrors.ErrorOldResultCompatible, string(result.OSBuildOutput.Error), nil)
		} else if len(result.TargetErrors()) > 0 {
			result.JobError = clienterrors.New(clienterrors.ErrorTargetError, "at least one target failed", result.TargetErrors())
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
		result.JobError = clienterrors.New(clienterrors.ErrorOldResultCompatible, result.KojiError, nil)
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
		result.JobError = clienterrors.New(clienterrors.ErrorOldResultCompatible, result.KojiError, nil)
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
			result.JobError = clienterrors.New(clienterrors.ErrorDNFDepsolveError, result.Error, nil)
		} else {
			result.JobError = clienterrors.New(clienterrors.ErrorRPMMDError, result.Error, nil)
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
		return nil, fmt.Errorf("expected %q, found %q job instead", JobTypeContainerResolve, jobInfo.JobType)
	}

	return jobInfo, nil
}

func (s *Server) FileResolveJobInfo(id uuid.UUID, result *FileResolveJobResult) (*JobInfo, error) {
	jobInfo, err := s.jobInfo(id, result)

	if err != nil {
		return nil, err
	}

	if jobInfo.JobType != JobTypeFileResolve {
		return nil, fmt.Errorf("expected %q, found %q job instead", JobTypeFileResolve, jobInfo.JobType)
	}

	return jobInfo, nil
}

func (s *Server) OSTreeResolveJobInfo(id uuid.UUID, result *OSTreeResolveJobResult) (*JobInfo, error) {
	jobInfo, err := s.jobInfo(id, result)
	if err != nil {
		return nil, err
	}

	if jobInfo.JobType != JobTypeOSTreeResolve {
		return nil, fmt.Errorf("expected %q, found %q job instead", JobTypeOSTreeResolve, jobInfo.JobType)
	}

	return jobInfo, nil
}

func (s *Server) AWSEC2CopyJobInfo(id uuid.UUID, result *AWSEC2CopyJobResult) (*JobInfo, error) {
	jobInfo, err := s.jobInfo(id, result)
	if err != nil {
		return nil, err
	}

	if jobInfo.JobType != JobTypeAWSEC2Copy {
		return nil, fmt.Errorf("expected %q, found %q job instead", JobTypeAWSEC2Copy, jobInfo.JobType)
	}

	return jobInfo, nil
}

func (s *Server) AWSEC2ShareJobInfo(id uuid.UUID, result *AWSEC2ShareJobResult) (*JobInfo, error) {
	jobInfo, err := s.jobInfo(id, result)
	if err != nil {
		return nil, err
	}

	if jobInfo.JobType != JobTypeAWSEC2Share {
		return nil, fmt.Errorf("expected %q, found %q job instead", JobTypeAWSEC2Share, jobInfo.JobType)
	}

	return jobInfo, nil
}

func (s *Server) jobInfo(id uuid.UUID, result interface{}) (*JobInfo, error) {
	jobType, channel, rawResult, queued, started, finished, canceled, deps, dependents, err := s.jobs.JobStatus(id)
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
		Deps:       deps,
		Dependents: dependents,
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

func (s *Server) RequestJob(ctx context.Context, arch string, jobTypes, channels []string, workerID uuid.UUID) (uuid.UUID, uuid.UUID, string, json.RawMessage, []json.RawMessage, error) {
	return s.requestJob(ctx, arch, jobTypes, uuid.Nil, channels, workerID)
}

func (s *Server) RequestJobById(ctx context.Context, arch string, requestedJobId uuid.UUID) (uuid.UUID, uuid.UUID, string, json.RawMessage, []json.RawMessage, error) {
	return s.requestJob(ctx, arch, []string{}, requestedJobId, nil, uuid.Nil)
}

func (s *Server) requestJob(ctx context.Context, arch string, jobTypes []string, requestedJobId uuid.UUID, channels []string, workerID uuid.UUID) (
	jobId uuid.UUID, token uuid.UUID, jobType string, args json.RawMessage, dynamicArgs []json.RawMessage, err error) {
	// treat osbuild jobs specially until we have found a generic way to
	// specify dequeuing restrictions. For now, we only have one
	// restriction: arch for osbuild jobs.
	jts := []string{}
	// Only set the label used for prometheus metrics when it's an osbuild job. Otherwise the
	// dequeue metrics would set the label for all job types, while the finish metrics only set
	// it for osbuild jobs.
	var archPromLabel string
	for _, t := range jobTypes {
		if t == JobTypeOSBuild {
			t = t + ":" + arch
			archPromLabel = arch
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
		token, depIDs, jobType, args, err = s.jobs.DequeueByID(dequeueCtx, requestedJobId, workerID)
	} else {
		jobId, token, depIDs, jobType, args, err = s.jobs.Dequeue(dequeueCtx, workerID, jts, channels)
	}
	if err != nil {
		if err != jobqueue.ErrDequeueTimeout && err != jobqueue.ErrNotPending {
			logrus.Errorf("dequeuing job failed: %v", err)
		}
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
		_, _, result, _, _, finished, _, _, _, err = s.jobs.JobStatus(depID)
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

	prometheus.DequeueJobMetrics(pending, jobInfo.JobStatus.Started, jobInfo.JobType, jobInfo.Channel, archPromLabel)

	return
}

func (s *Server) FinishJob(token uuid.UUID, result json.RawMessage) error {
	return s.RequeueOrFinishJob(token, 0, result)
}

func (s *Server) RequeueOrFinishJob(token uuid.UUID, maxRetries uint64, result json.RawMessage) error {
	jobId, err := s.jobs.IdFromToken(token)
	if err != nil {
		switch err {
		case jobqueue.ErrNotExist:
			return ErrInvalidToken
		default:
			return err
		}
	}

	requeued, err := s.jobs.RequeueOrFinishJob(jobId, maxRetries, result)
	if err != nil {
		switch err {
		case jobqueue.ErrNotRunning:
			return ErrJobNotRunning
		default:
			return fmt.Errorf("error finishing job: %v", err)
		}
	}

	if requeued {
		jobInfo, err := s.jobInfo(jobId, nil)
		if err != nil {
			return fmt.Errorf("error requeueing job: %w", err)
		}
		prometheus.RequeueJobMetrics(jobInfo.JobType, jobInfo.Channel)
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
	case JobTypeAWSEC2Copy:
		var awsEC2CopyJR AWSEC2CopyJobResult
		jobInfo, err = s.AWSEC2CopyJobInfo(jobId, &awsEC2CopyJR)
		if err != nil {
			return err
		}
		jobResult = &awsEC2CopyJR.JobResult
	case JobTypeAWSEC2Share:
		var awsEC2ShareJR AWSEC2ShareJobResult
		jobInfo, err = s.AWSEC2ShareJobInfo(jobId, &awsEC2ShareJR)
		if err != nil {
			return err
		}
		jobResult = &awsEC2ShareJR.JobResult
	case JobTypeContainerResolve:
		var containerResolveJR ContainerResolveJobResult
		jobInfo, err = s.ContainerResolveJobInfo(jobId, &containerResolveJR)
		if err != nil {
			return err
		}
		jobResult = &containerResolveJR.JobResult
	case JobTypeFileResolve:
		var fileResolveJR FileResolveJobResult
		jobInfo, err = s.FileResolveJobInfo(jobId, &fileResolveJR)
		if err != nil {
			return err
		}
		jobResult = &fileResolveJR.JobResult
	case JobTypeOSTreeResolve:
		var ostreeResolveJR OSTreeResolveJobResult
		jobInfo, err = s.OSTreeResolveJobInfo(jobId, &ostreeResolveJR)
		if err != nil {
			return err
		}
		jobResult = &ostreeResolveJR.JobResult

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

func (s *Server) RegisterWorker(c, a string) (uuid.UUID, error) {
	workerID, err := s.jobs.InsertWorker(c, a)
	if err != nil {
		return uuid.Nil, err
	}
	logrus.Infof("Worker (%v) registered", a)
	return workerID, nil
}

func (s *Server) WorkerAvailableForArch(a string) (bool, error) {
	workers, err := s.jobs.Workers(0)
	if err != nil {
		return false, err
	}
	for _, w := range workers {
		if a == w.Arch {
			return true, nil
		}
	}
	return false, nil
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

	workerID := uuid.Nil
	if body.WorkerId != nil {
		workerID, err = uuid.Parse(*body.WorkerId)
		if err != nil {
			return api.HTTPErrorWithInternal(api.ErrorMalformedWorkerId, err)
		}
	}

	jobId, jobToken, jobType, jobArgs, dynamicJobArgs, err := h.server.RequestJob(ctx.Request().Context(), body.Arch, body.Types, []string{channel}, workerID)
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

func (h *apiHandlers) PostWorkers(ctx echo.Context) error {
	var body api.PostWorkersRequest
	err := ctx.Bind(&body)
	if err != nil {
		return err
	}

	var channel string
	if h.server.config.JWTEnabled {
		tenant, err := auth.GetFromClaims(ctx.Request().Context(), h.server.config.TenantProviderFields)
		if err != nil {
			return api.HTTPErrorWithInternal(api.ErrorTenantNotFound, err)
		}

		// prefix the tenant to prevent collisions if support for specifying channels in a request is ever added
		channel = "org-" + tenant
	}

	workerID, err := h.server.RegisterWorker(channel, body.Arch)
	if err != nil {
		return api.HTTPErrorWithInternal(api.ErrorInsertingWorker, err)
	}

	return ctx.JSON(http.StatusCreated, api.PostWorkersResponse{
		ObjectReference: api.ObjectReference{
			Href: fmt.Sprintf("%s/workers", api.BasePath),
			Id:   workerID.String(),
			Kind: "WorkerID",
		},
		WorkerId: workerID.String(),
	})
}

func (h *apiHandlers) PostWorkerStatus(ctx echo.Context, workerIdstr string) error {
	workerID, err := uuid.Parse(workerIdstr)
	if err != nil {
		return api.HTTPErrorWithInternal(api.ErrorMalformedWorkerId, err)
	}
	err = h.server.jobs.UpdateWorkerStatus(workerID)

	if err == jobqueue.ErrWorkerNotExist {
		return api.HTTPErrorWithInternal(api.ErrorWorkerIdNotFound, err)
	}

	if err != nil {
		return api.HTTPErrorWithInternal(api.ErrorUpdatingWorkerStatus, err)
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
