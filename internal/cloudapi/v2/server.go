package v2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/sbom"
	"github.com/osbuild/osbuild-composer/internal/auth"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/prometheus"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

// How long to wait for a depsolve job to finish
const depsolveTimeoutMin = 5

// serializeManifestFunc is used to serialize the manifest
// it can be overridden for testing
var serializeManifestFunc = serializeManifest

// Server represents the state of the cloud Server
type Server struct {
	workers *worker.Server
	distros *distrofactory.Factory
	repos   *reporegistry.RepoRegistry
	config  ServerConfig
	router  routers.Router

	goroutinesCtx       context.Context
	goroutinesCtxCancel context.CancelFunc
	goroutinesGroup     sync.WaitGroup
}

type ServerConfig struct {
	TenantProviderFields []string
	JWTEnabled           bool

	// Experimental configuration option. Can only be set through the
	// IMAGE_BUILDER_EXPERIMENTAL environment variable using:
	//
	//     IMAGE_BUILDER_EXPERIMENTAL="image-builder-manifest-generation"
	ImageBuilderManifestGeneration bool
}

func NewServer(workers *worker.Server, distros *distrofactory.Factory, repos *reporegistry.RepoRegistry, config ServerConfig) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	spec, err := GetSwagger()
	if err != nil {
		panic(err)
	}

	loader := openapi3.NewLoader()
	if err := spec.Validate(loader.Context); err != nil {
		panic(err)
	}

	router, err := legacyrouter.NewRouter(spec)
	if err != nil {
		panic(err)
	}

	server := &Server{
		workers: workers,
		distros: distros,
		repos:   repos,
		config:  config,
		router:  router,

		goroutinesCtx:       ctx,
		goroutinesCtxCancel: cancel,
	}
	return server
}

func (s *Server) Handler(path string) http.Handler {
	e := echo.New()
	e.Binder = binder{}
	e.HTTPErrorHandler = HTTPErrorHandler
	e.Logger = common.Logger()

	// OperationIDMiddleware - generates OperationID random string and puts it into the contexts
	// ExternalIDMiddleware - extracts ID from HTTP header and puts it into the contexts
	// LoggerMiddleware - creates context-aware logger for each request
	e.Pre(common.OperationIDMiddleware, common.ExternalIDMiddleware, common.LoggerMiddleware)
	e.Use(middleware.Recover())

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:     true,
		LogStatus:  true,
		LogLatency: true,
		LogMethod:  true,
		LogValuesFunc: func(c echo.Context, values middleware.RequestLoggerValues) error {
			fields := logrus.Fields{
				"uri":          values.URI,
				"method":       values.Method,
				"status":       values.Status,
				"latency_ms":   values.Latency.Milliseconds(),
				"operation_id": c.Get(common.OperationIDKey),
				"external_id":  c.Get(common.ExternalIDKey),
			}
			if values.Error != nil {
				fields["error"] = values.Error
			}
			logrus.WithFields(fields).Infof("Processed request %s %s", values.Method, values.URI)

			return nil
		},
	}))

	if sentry.CurrentHub().Client() == nil {
		logrus.Warn("Sentry/Glitchtip not initialized, echo middleware was not enabled")
	} else {
		e.Use(sentryecho.New(sentryecho.Options{}))
	}

	handler := apiHandlers{
		server: s,
	}

	mws := []echo.MiddlewareFunc{
		prometheus.StatusMiddleware(prometheus.ComposerSubsystem),
	}
	if s.config.JWTEnabled {
		mws = append(mws, auth.TenantChannelMiddleware(s.config.TenantProviderFields, HTTPError(ErrorTenantNotFound)))
	}
	mws = append(mws,
		prometheus.HTTPDurationMiddleware(prometheus.ComposerSubsystem),
		prometheus.MetricsMiddleware, s.ValidateRequest)
	RegisterHandlers(e.Group(path, mws...), &handler)

	return e
}

func (s *Server) Shutdown() {
	s.goroutinesCtxCancel()
	s.goroutinesGroup.Wait()
}

type manifestJobDependencies struct {
	depsolveJobID         uuid.UUID
	containerResolveJobID uuid.UUID
	ostreeResolveJobID    uuid.UUID
}

// IDs returns a slice of the non-nil job IDs.
func (mjd manifestJobDependencies) IDs() []uuid.UUID {
	var ids []uuid.UUID
	if mjd.depsolveJobID != uuid.Nil {
		ids = append(ids, mjd.depsolveJobID)
	}
	if mjd.containerResolveJobID != uuid.Nil {
		ids = append(ids, mjd.containerResolveJobID)
	}
	if mjd.ostreeResolveJobID != uuid.Nil {
		ids = append(ids, mjd.ostreeResolveJobID)
	}
	return ids
}

// enqueueResolveJobs adds all the necessary content resolve jobs for the
// manifest to the queue and returns a [manifestJobDependencies] that holds
// resolve job IDs by type.
func (s *Server) enqueueResolveJobs(manifestSource *manifest.Manifest, it distro.ImageType, channel string) (manifestJobDependencies, error) {
	var jobDependencies manifestJobDependencies

	arch := it.Arch()
	distribution := arch.Distro()

	pkgSetChains, err := manifestSource.GetPackageSetChains()
	if err != nil {
		return jobDependencies, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}
	depsolveJobID, err := s.workers.EnqueueDepsolve(&worker.DepsolveJob{
		PackageSets:      pkgSetChains,
		ModulePlatformID: distribution.ModulePlatformID(),
		Arch:             arch.Name(),
		Releasever:       distribution.Releasever(),
		SbomType:         sbom.StandardTypeSpdx,
	}, channel)
	if err != nil {
		return jobDependencies, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}
	jobDependencies.depsolveJobID = depsolveJobID

	containerSources := manifestSource.GetContainerSourceSpecs()
	if len(containerSources) > 1 {
		// only one pipeline can embed containers
		pipelines := make([]string, 0, len(containerSources))
		for name := range containerSources {
			pipelines = append(pipelines, name)
		}
		return jobDependencies, HTTPErrorWithInternal(ErrorEnqueueingJob, fmt.Errorf("manifest returned %d pipelines with containers (at most 1 is supported): %s", len(containerSources), strings.Join(pipelines, ", ")))
	}

	for _, sources := range containerSources {
		workerResolveSpecs := make([]worker.ContainerSpec, len(sources))
		for idx, source := range sources {
			workerResolveSpecs[idx] = worker.ContainerSpec{
				Source:    source.Source,
				Name:      source.Name,
				TLSVerify: source.TLSVerify,
			}
		}

		job := worker.ContainerResolveJob{
			Arch:  arch.Name(),
			Specs: workerResolveSpecs,
		}

		containerResolveJobID, err := s.workers.EnqueueContainerResolveJob(&job, channel)
		if err != nil {
			return jobDependencies, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}

		jobDependencies.containerResolveJobID = containerResolveJobID
		break // there can be only one
	}

	commitSources := manifestSource.GetOSTreeSourceSpecs()
	if len(commitSources) > 1 {
		// only one pipeline can specify an ostree commit for content
		pipelines := make([]string, 0, len(commitSources))
		for name := range commitSources {
			pipelines = append(pipelines, name)
		}
		return jobDependencies, HTTPErrorWithInternal(ErrorEnqueueingJob, fmt.Errorf("manifest returned %d pipelines with ostree commits (at most 1 is supported): %s", len(commitSources), strings.Join(pipelines, ", ")))
	}
	for _, sources := range commitSources {
		workerResolveSpecs := make([]worker.OSTreeResolveSpec, len(sources))
		for idx, source := range sources {
			// ostree.SourceSpec is directly convertible to worker.OSTreeResolveSpec
			workerResolveSpecs[idx] = worker.OSTreeResolveSpec{
				URL:  source.URL,
				Ref:  source.Ref,
				RHSM: source.RHSM,
			}

		}
		ostreeResolveJobID, err := s.workers.EnqueueOSTreeResolveJob(&worker.OSTreeResolveJob{Specs: workerResolveSpecs}, channel)
		if err != nil {
			return jobDependencies, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}

		jobDependencies.ostreeResolveJobID = ostreeResolveJobID
		break // there can be only one
	}

	return jobDependencies, nil
}

func (s *Server) enqueueCompose(irs []imageRequest, channel string) (uuid.UUID, error) {
	var id uuid.UUID
	if len(irs) != 1 {
		return id, HTTPError(ErrorInvalidNumberOfImageBuilds)
	}
	ir := irs[0]

	manifestSource, _, err := ir.imageType.Manifest(&ir.blueprint, ir.imageOptions, ir.repositories, &ir.manifestSeed)
	if err != nil {
		logrus.Warningf("ErrorEnqueueingJob, failed generating manifest: %v", err)
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	dependencies, err := s.enqueueResolveJobs(manifestSource, ir.imageType, channel)
	if err != nil {
		logrus.Warningf("ErrorEnqueueingJob, failed creating resolve jobs: %v", err)
		return id, err
	}

	manifestJobID, err := s.workers.EnqueueManifestJobByID(&worker.ManifestJobByID{}, dependencies.IDs(), channel)
	if err != nil {
		logrus.Warningf("ErrorEnqueueingJob, failed creating manifest job (ByID): %v", err)
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	id, err = s.workers.EnqueueOSBuildAsDependency(ir.imageType.Arch().Name(), &worker.OSBuildJob{
		Targets: ir.targets,
		PipelineNames: &worker.PipelineNames{
			Build:   manifestSource.BuildPipelines(),
			Payload: manifestSource.PayloadPipelines(),
		},
	}, []uuid.UUID{manifestJobID}, channel)
	if err != nil {
		logrus.Warningf("ErrorEnqueueingJob, failed creating osbuild job: %v", err)
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	s.goroutinesGroup.Add(1)
	go func() {
		serializeManifestFunc(s.goroutinesCtx, manifestSource, s.workers, dependencies, manifestJobID, ir.manifestSeed)
		defer s.goroutinesGroup.Done()
	}()

	return id, nil
}

func (s *Server) enqueueComposeIBCLI(irs []imageRequest, channel string) (uuid.UUID, error) {
	var osbuildJobID uuid.UUID
	if len(irs) != 1 {
		return osbuildJobID, HTTPErrorWithInternal(ErrorInvalidNumberOfImageBuilds, fmt.Errorf("expected 1 image request, got %d", len(irs)))
	}
	ir := irs[0]

	arch := ir.imageType.Arch()
	distribution := arch.Distro()
	imageType := ir.imageType

	rawBlueprint, err := json.Marshal(ir.blueprint)
	if err != nil {
		return osbuildJobID, HTTPErrorWithInternal(ErrorJSONMarshallingError, fmt.Errorf("failed to marshal blueprint for image-builder-manifest job"))
	}

	args := worker.ImageBuilderArgs{
		Distro:       distribution.Name(),
		Arch:         arch.Name(),
		ImageType:    imageType.Name(),
		Blueprint:    rawBlueprint,
		Repositories: ir.repositories,
	}

	manifestJob := worker.ImageBuilderManifestJob{
		Args: args,

		// NOTE: image-builder doesn't support setting the rpmmd cache and tries to
		// read XDG_CACHE_HOME, which fails when running as _osbuild-composer.
		// Once https://github.com/osbuild/image-builder-cli/pull/358 is
		// merged, we can use the new --rpmmd-cache option in the image-builder
		// call instead.
		// TODO: make sure we use the same rpmmd cache that's used by the depsolve
		// job for consistency
		ExtraEnv: []string{"XDG_CACHE_HOME=/var/cache/osbuild-composer/rpmmd"},
	}

	manifestJobID, err := s.workers.EnqueueImageBuilderManifestJob(&manifestJob, channel)
	if err != nil {
		return osbuildJobID, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}
	logrus.Debugf("manifest job enqueued: %v", manifestJobID)

	osbuildJobID, err = s.workers.EnqueueOSBuildAsDependency(arch.Name(),
		&worker.OSBuildJob{
			Targets:       ir.targets,
			PipelineNames: nil, // NOTE: the manifest job result provides these when they're not available from the osbuild job args
		}, []uuid.UUID{manifestJobID}, channel)
	if err != nil {
		return osbuildJobID, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}
	logrus.Debugf("osbuild job enqueued: %v (with dep %v)", osbuildJobID, manifestJobID)

	return osbuildJobID, nil
}

func (s *Server) enqueueKojiCompose(taskID uint64, server, name, version, release string, irs []imageRequest, channel string) (uuid.UUID, error) {
	var id uuid.UUID
	kojiDirectory := "osbuild-cg/osbuild-composer-koji-" + uuid.New().String()

	initID, err := s.workers.EnqueueKojiInit(&worker.KojiInitJob{
		Server:  server,
		Name:    name,
		Version: version,
		Release: release,
	}, channel)
	if err != nil {
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	var kojiFilenames []string
	var buildIDs []uuid.UUID
	for idx, ir := range irs {

		manifestSource, _, err := ir.imageType.Manifest(&ir.blueprint, ir.imageOptions, ir.repositories, &irs[idx].manifestSeed)
		if err != nil {
			logrus.Errorf("ErrorEnqueueingJob, failed generating manifest: %v", err)
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}

		dependencies, err := s.enqueueResolveJobs(manifestSource, ir.imageType, channel)
		if err != nil {
			logrus.Warningf("ErrorEnqueueingJob, failed creating resolve jobs: %v", err)
			return id, err
		}

		manifestJobID, err := s.workers.EnqueueManifestJobByID(&worker.ManifestJobByID{}, dependencies.IDs(), channel)
		if err != nil {
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}

		archName := ir.imageType.Arch().Name()
		kojiFilename := fmt.Sprintf(
			"%s-%s-%s.%s%s",
			name,
			version,
			release,
			archName,
			splitExtension(ir.imageType.Filename()),
		)

		kojiTarget := target.NewKojiTarget(&target.KojiTargetOptions{
			Server:          server,
			UploadDirectory: kojiDirectory,
		})
		kojiTarget.OsbuildArtifact.ExportFilename = ir.imageType.Filename()
		kojiTarget.OsbuildArtifact.ExportName = ir.imageType.Exports()[0]
		kojiTarget.ImageName = kojiFilename

		targets := []*target.Target{kojiTarget}
		// add any cloud upload targets if defined
		if ir.targets != nil {
			targets = append(targets, ir.targets...)
		}

		buildID, err := s.workers.EnqueueOSBuildAsDependency(archName, &worker.OSBuildJob{
			PipelineNames: &worker.PipelineNames{
				Build:   manifestSource.BuildPipelines(),
				Payload: manifestSource.PayloadPipelines(),
			},
			Targets:            targets,
			ManifestDynArgsIdx: common.ToPtr(1),
			DepsolveDynArgsIdx: common.ToPtr(2),
			ImageBootMode:      ir.imageType.BootMode().String(),
		}, []uuid.UUID{initID, manifestJobID, dependencies.depsolveJobID}, channel)
		if err != nil {
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}
		kojiFilenames = append(kojiFilenames, kojiFilename)
		buildIDs = append(buildIDs, buildID)

		// copy the image request while passing it into the goroutine to prevent data races
		s.goroutinesGroup.Add(1)
		go func(ir imageRequest) {
			serializeManifestFunc(s.goroutinesCtx, manifestSource, s.workers, dependencies, manifestJobID, ir.manifestSeed)
			defer s.goroutinesGroup.Done()
		}(ir)
	}
	id, err = s.workers.EnqueueKojiFinalize(&worker.KojiFinalizeJob{
		Server:        server,
		Name:          name,
		Version:       version,
		Release:       release,
		KojiFilenames: kojiFilenames,
		KojiDirectory: kojiDirectory,
		TaskID:        taskID,
		StartTime:     uint64(time.Now().Unix()), // nolint: gosec
	}, initID, buildIDs, channel)
	if err != nil {
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	return id, nil
}

func serializeManifest(ctx context.Context, manifestSource *manifest.Manifest, workers *worker.Server, dependencies manifestJobDependencies, manifestJobID uuid.UUID, seed int64) {
	// prepared to become a config variable
	ctx, cancel := context.WithTimeout(ctx, time.Minute*depsolveTimeoutMin)
	defer cancel()

	jobResult := &worker.ManifestJobByIDResult{
		Manifest: nil,
		ManifestInfo: worker.ManifestInfo{
			OSBuildComposerVersion: common.BuildVersion(),
		},
	}

	var dynArgs []json.RawMessage
	var err error
	token := uuid.Nil
	logWithId := logrus.WithField("jobId", manifestJobID)

	defer func() {
		if r := recover(); r != nil {
			logWithId.Errorf("Recovered from panic in serializeManifest: %v", r)
			jobResult.JobError = clienterrors.New(clienterrors.ErrorManifestGeneration, "Error serializing manifest", r)
		}

		// token == uuid.Nil indicates that no worker even started processing
		if token == uuid.Nil {
			if jobResult.JobError != nil {
				// set all jobs to "failed"
				// osbuild job will fail as dependency
				jobs := []struct {
					Name string
					ID   uuid.UUID
				}{
					{"depsolve", dependencies.depsolveJobID},
					{"containerResolve", dependencies.containerResolveJobID},
					{"ostreeResolve", dependencies.ostreeResolveJobID},
					{"manifest", manifestJobID},
				}

				for _, job := range jobs {
					if job.ID != uuid.Nil {
						err := workers.SetFailed(job.ID, jobResult.JobError)
						if err != nil {
							logWithId.Errorf("Error failing %s job: %v", job.Name, err)
						}
					}
				}

			} else {
				logWithId.Errorf("Internal error, no worker started depsolve but we didn't get a reason.")
			}
		} else {
			result, err := json.Marshal(jobResult)
			if err != nil {
				logWithId.Errorf("Error marshalling manifest job results: %v", err)
			}
			err = workers.FinishJob(token, result)
			if err != nil {
				logWithId.Errorf("Error finishing manifest job: %v", err)
			}
			if jobResult.JobError != nil {
				logWithId.Errorf("Error in manifest job %v: %v", jobResult.JobError.Reason, err)
			}
		}
	}()

	// wait until job is in a pending state
	for {
		_, token, _, _, dynArgs, err = workers.RequestJobById(ctx, "", manifestJobID)
		if errors.Is(err, jobqueue.ErrNotPending) {
			logWithId.Debug("Manifest job not pending, waiting for depsolve job to finish")
			time.Sleep(time.Millisecond * 50)
			select {
			case <-ctx.Done():
				logWithId.Warning(fmt.Sprintf("Manifest job dependencies took longer than %d minutes to finish,"+
					" or the server is shutting down, returning to avoid dangling routines", depsolveTimeoutMin))

				jobResult.JobError = clienterrors.New(clienterrors.ErrorDepsolveTimeout,
					"Timeout while waiting for package dependency resolution",
					"There may be a temporary issue with compute resources.",
				)
				break
			default:
				continue
			}
		}
		if err != nil {
			logWithId.Errorf("Error requesting manifest job: %v", err)
			return
		}
		break
	}

	// add osbuild/images dependency info to job result
	osbuildImagesDep, err := common.GetDepModuleInfoByPath(common.OSBuildImagesModulePath)
	if err != nil {
		// do not fail here and just log the error, because the module info is not available in tests.
		// Failing here would make the unit tests fail. See https://github.com/golang/go/issues/33976
		logWithId.Errorf("Error getting %s dependency info: %v", common.OSBuildImagesModulePath, err)
	} else {
		osbuildImagesDepModule := worker.ComposerDepModuleFromDebugModule(osbuildImagesDep)
		jobResult.ManifestInfo.OSBuildComposerDeps = append(jobResult.ManifestInfo.OSBuildComposerDeps, osbuildImagesDepModule)
	}

	if len(dynArgs) == 0 {
		reason := "No dynamic arguments"
		jobResult.JobError = clienterrors.New(clienterrors.ErrorNoDynamicArgs, reason, nil)
		return
	}

	var depsolveResults worker.DepsolveJobResult
	err = json.Unmarshal(dynArgs[0], &depsolveResults)
	if err != nil {
		reason := "Error parsing dynamic arguments"
		jobResult.JobError = clienterrors.New(clienterrors.ErrorParsingDynamicArgs, reason, nil)
		return
	}

	_, err = workers.DepsolveJobInfo(dependencies.depsolveJobID, &depsolveResults)
	if err != nil {
		reason := "Error reading depsolve status"
		jobResult.JobError = clienterrors.New(clienterrors.ErrorReadingJobStatus, reason, nil)
		return
	}

	if jobErr := depsolveResults.JobError; jobErr != nil {
		if jobErr.ID == clienterrors.ErrorDNFDepsolveError || jobErr.ID == clienterrors.ErrorDNFMarkingErrors {
			jobResult.JobError = clienterrors.New(clienterrors.ErrorDepsolveDependency, "Error in depsolve job dependency input, bad package set requested", jobErr.Details)
			return
		}
		jobResult.JobError = clienterrors.New(clienterrors.ErrorDepsolveDependency, "Error in depsolve job dependency", jobErr.Details)
		return
	}

	if len(depsolveResults.PackageSpecs) == 0 {
		jobResult.JobError = clienterrors.New(clienterrors.ErrorEmptyPackageSpecs, "Received empty package specs", nil)
		return
	}

	var containerSpecs map[string][]container.Spec
	if dependencies.containerResolveJobID != uuid.Nil {
		// Container resolve job
		var result worker.ContainerResolveJobResult
		_, err := workers.ContainerResolveJobInfo(dependencies.containerResolveJobID, &result)

		if err != nil {
			reason := "Error reading container resolve job status"
			jobResult.JobError = clienterrors.New(clienterrors.ErrorReadingJobStatus, reason, nil)
			return
		}

		if jobErr := result.JobError; jobErr != nil {
			jobResult.JobError = clienterrors.New(clienterrors.ErrorContainerDependency, "Error in container resolve job dependency", nil)
			return
		}

		// NOTE: The container resolve job doesn't hold the pipeline name for
		// the container embedding, so we need to get it from the manifest
		// content field. There should be only one.
		var containerEmbedPipeline string
		for name := range manifestSource.GetContainerSourceSpecs() {
			containerEmbedPipeline = name
			break
		}

		pipelineSpecs := make([]container.Spec, len(result.Specs))
		for idx, resultSpec := range result.Specs {
			pipelineSpecs[idx] = container.Spec{
				Source:     resultSpec.Source,
				Digest:     resultSpec.Digest,
				LocalName:  resultSpec.Name,
				TLSVerify:  resultSpec.TLSVerify,
				ImageID:    resultSpec.ImageID,
				ListDigest: resultSpec.ListDigest,
			}

		}
		containerSpecs = map[string][]container.Spec{
			containerEmbedPipeline: pipelineSpecs,
		}
	}

	var ostreeCommitSpecs map[string][]ostree.CommitSpec
	if dependencies.ostreeResolveJobID != uuid.Nil {
		var result worker.OSTreeResolveJobResult
		_, err := workers.OSTreeResolveJobInfo(dependencies.ostreeResolveJobID, &result)

		if err != nil {
			reason := "Error reading ostree resolve job status"
			logrus.Errorf("%s: %v", reason, err)
			jobResult.JobError = clienterrors.New(clienterrors.ErrorReadingJobStatus, reason, nil)
			return
		}

		if jobErr := result.JobError; jobErr != nil {
			jobResult.JobError = clienterrors.New(clienterrors.ErrorOSTreeDependency, "Error in ostree resolve job dependency", nil)
			return
		}

		// NOTE: The ostree resolve job doesn't hold the pipeline name for the
		// ostree commits, so we need to get it from the manifest content
		// field. There should be only one.
		var ostreeCommitPipeline string
		for name := range manifestSource.GetOSTreeSourceSpecs() {
			ostreeCommitPipeline = name
			break
		}

		commitSpecs := make([]ostree.CommitSpec, len(result.Specs))
		for idx, resultSpec := range result.Specs {
			commitSpecs[idx] = ostree.CommitSpec{
				Ref:      resultSpec.Ref,
				URL:      resultSpec.URL,
				Checksum: resultSpec.Checksum,
				Secrets:  resultSpec.Secrets,
			}
		}
		ostreeCommitSpecs = map[string][]ostree.CommitSpec{
			ostreeCommitPipeline: commitSpecs,
		}
	}

	// XXX: fix worker.DepsolveJobResult
	depsolveResultsInTheRightFormat := map[string]depsolvednf.DepsolveResult{}
	for plName, res := range depsolveResults.PackageSpecs {
		r := depsolveResultsInTheRightFormat[plName]
		r.Packages = res.ToRPMMDList()
		depsolveResultsInTheRightFormat[plName] = r
	}
	for plName, res := range depsolveResults.RepoConfigs {
		r := depsolveResultsInTheRightFormat[plName]
		r.Repos = res
		depsolveResultsInTheRightFormat[plName] = r
	}
	ms, err := manifestSource.Serialize(depsolveResultsInTheRightFormat, containerSpecs, ostreeCommitSpecs, nil)
	if err != nil {
		reason := "Error serializing manifest"
		jobResult.JobError = clienterrors.New(clienterrors.ErrorManifestGeneration, reason, err.Error())
		return
	}

	jobResult.Manifest = ms
}
