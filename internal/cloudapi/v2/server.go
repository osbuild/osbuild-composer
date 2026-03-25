package v2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
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

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/generic"
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

// maxJobTimeoutMinutes is the maximum timeout in minutes for a job to finish
const maxJobTimeoutMinutes = 5

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

	server.goroutinesGroup.Add(1)
	go func() {
		defer server.goroutinesGroup.Done()
		server.bootcPreManifestLoop()
	}()

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
		LogURI:          true,
		LogStatus:       true,
		LogLatency:      true,
		LogMethod:       true,
		LogResponseSize: true,
		LogValuesFunc: func(c echo.Context, values middleware.RequestLoggerValues) error {
			fields := logrus.Fields{
				"uri":                 values.URI,
				"method":              values.Method,
				"status":              values.Status,
				"latency_ms":          values.Latency.Milliseconds(),
				"operation_id":        c.Get(common.OperationIDKey),
				"external_id":         c.Get(common.ExternalIDKey),
				"request_body_bytes":  c.Request().ContentLength,
				"response_body_bytes": values.ResponseSize,
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
	if len(containerSources) > 0 {
		pipelineSpecs := make(map[string][]worker.ContainerSpec, len(containerSources))
		for name, sources := range containerSources {
			specs := make([]worker.ContainerSpec, len(sources))
			for idx, source := range sources {
				specs[idx] = worker.ContainerSpecFromVendorSourceSpec(source)
			}
			pipelineSpecs[name] = specs
		}

		job := worker.ContainerResolveJob{
			Arch:          arch.Name(),
			PipelineSpecs: pipelineSpecs,
		}

		containerResolveJobID, err := s.workers.EnqueueContainerResolveJob(&job, channel)
		if err != nil {
			return jobDependencies, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}
		jobDependencies.containerResolveJobID = containerResolveJobID
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
	logrus.Warnf("using experimental job type: %s", worker.JobTypeImageBuilderManifest)
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
		Subscription: ir.imageOptions.Subscription,
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

func (s *Server) enqueueBootcCompose(request ComposeRequest, channel string) (uuid.UUID, error) {
	var ir ImageRequest
	if request.ImageRequest != nil {
		ir = *request.ImageRequest
	} else if request.ImageRequests != nil && len(*request.ImageRequests) == 1 {
		ir = (*request.ImageRequests)[0]
	} else {
		return uuid.Nil, HTTPError(ErrorInvalidNumberOfImageBuilds)
	}

	bp, err := request.GetBlueprint()
	if err != nil {
		return uuid.Nil, err
	}

	mani, err := s.workers.EnqueueBootcManifestJob(&worker.BootcManifestJob{
		Ref:       request.Bootc.Reference,
		BuildRef:  request.Bootc.Reference,
		Arch:      ir.Architecture,
		ImageType: imageTypeFromApiImageType(ir.ImageType),
		Blueprint: bp,
	}, channel)
	if err != nil {
		return uuid.Nil, err
	}

	// Only local targets are supported
	if ir.UploadTargets != nil {
		for _, ut := range *ir.UploadTargets {
			if ut.Type != UploadTypesLocal {
				return uuid.Nil, HTTPError(ErrorInvalidUploadTarget)
			}
		}
	}
	tgts := []*target.Target{
		target.NewWorkerServerTarget(),
	}

	// TODO: Hardcoding this is obviously a very bad hack, but currently this image type
	// information isn't retrievable from images without running the bootc container.
	if imageTypeFromApiImageType(ir.ImageType) == "qcow2" {
		tgts[0].ImageName = "disk.qcow2"
		tgts[0].OsbuildArtifact.ExportFilename = "disk.qcow2"
		tgts[0].OsbuildArtifact.ExportName = "qcow2"
	} else {
		return uuid.Nil, HTTPErrorWithDetails(ErrorUnsupportedImageType, nil, "only qcow2 (guest-image) is supported for bootc composes")
	}
	id, err := s.workers.EnqueueOSBuildAsDependency(ir.Architecture, &worker.OSBuildJob{
		Targets: tgts,
	}, []uuid.UUID{mani}, channel)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func serializeManifest(ctx context.Context, manifestSource *manifest.Manifest, workers *worker.Server, dependencies manifestJobDependencies, manifestJobID uuid.UUID, seed int64) {
	// prepared to become a config variable
	ctx, cancel := context.WithTimeout(ctx, time.Minute*maxJobTimeoutMinutes)
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
				allJobIDs := slices.Concat(dependencies.IDs(), []uuid.UUID{manifestJobID})
				for _, jobID := range allJobIDs {
					err := workers.SetFailed(jobID, jobResult.JobError)
					if err != nil {
						logWithId.Errorf("Error failing job %s: %v", jobID, err)
					}
				}

			} else {
				logWithId.Errorf("Internal error, no worker started processing dependencies but we didn't get a reason.")
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
			logWithId.Debug("Manifest job not pending, waiting for dependencies to finish")
			time.Sleep(time.Millisecond * 50)
			select {
			case <-ctx.Done():
				logWithId.Warning(fmt.Sprintf("Manifest job dependencies took longer than %d minutes to finish,"+
					" or the server is shutting down, returning to avoid dangling routines", maxJobTimeoutMinutes))

				jobResult.JobError = clienterrors.New(clienterrors.ErrorJobDependency,
					"Timeout while waiting for dependencies to finish",
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

	var depsolveResult map[string]depsolvednf.DepsolveResult
	if dependencies.depsolveJobID != uuid.Nil {
		var depsolveJobResult worker.DepsolveJobResult
		_, err = workers.DepsolveJobInfo(dependencies.depsolveJobID, &depsolveJobResult)
		if err != nil {
			reason := "Error reading depsolve status"
			jobResult.JobError = clienterrors.New(clienterrors.ErrorReadingJobStatus, reason, nil)
			return
		}

		if jobErr := depsolveJobResult.JobError; jobErr != nil {
			if jobErr.ID == clienterrors.ErrorDNFDepsolveError || jobErr.ID == clienterrors.ErrorDNFMarkingErrors {
				jobResult.JobError = clienterrors.New(clienterrors.ErrorDepsolveDependency, "Error in depsolve job dependency input, bad package set requested", jobErr.Details)
				return
			}
			jobResult.JobError = clienterrors.New(clienterrors.ErrorDepsolveDependency, "Error in depsolve job dependency", jobErr.Details)
			return
		}

		if len(depsolveJobResult.PackageSpecs) == 0 {
			jobResult.JobError = clienterrors.New(clienterrors.ErrorEmptyPackageSpecs, "Received empty package specs", nil)
			return
		}

		depsolveResult, err = depsolveJobResult.ToDepsolvednfResult()
		if err != nil {
			reason := "Error converting depsolve result"
			jobResult.JobError = clienterrors.New(clienterrors.ErrorManifestGeneration, reason, err.Error())
			return
		}
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

		if result.PipelineSpecs != nil {
			containerSpecs = make(map[string][]container.Spec, len(result.PipelineSpecs))
			for name, specs := range result.PipelineSpecs {
				vendorSpecs := make([]container.Spec, len(specs))
				for i, s := range specs {
					vendorSpecs[i] = s.ToVendorSpec()
				}
				containerSpecs[name] = vendorSpecs
			}
		} else if result.Specs != nil {
			// TODO (2026-03-30, thozza): remove this once all workers are migrated to the new format.
			// Old worker fallback: flat result, reconstruct pipeline mapping using manifest source specs.
			containerSpecs, err = matchContainerSpecsToPipelines(result.Specs, manifestSource.GetContainerSourceSpecs())
			if err != nil {
				reason := "Error matching container specs to pipelines"
				jobResult.JobError = clienterrors.New(clienterrors.ErrorContainerDependency, reason, err.Error())
				return
			}
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

	ms, err := manifestSource.Serialize(depsolveResult, containerSpecs, ostreeCommitSpecs, nil)
	if err != nil {
		reason := "Error serializing manifest"
		jobResult.JobError = clienterrors.New(clienterrors.ErrorManifestGeneration, reason, err.Error())
		return
	}

	jobResult.Manifest = ms
}

// bootcPreManifestLoop is a long-running goroutine started at server init
// that picks up pending BootcPreManifest jobs via RequestJob and spawns a
// goroutine per job for parallel processing.
func (s *Server) bootcPreManifestLoop() {
	const maxConcurrentPreManifestJobs = 8
	sem := make(chan struct{}, maxConcurrentPreManifestJobs)

	// exponential backoff for dequeue errors
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		sem <- struct{}{}

		// Pass empty channel for on-prem (no tenant).
		// TODO: Multi-tenant deployments with JWT-based channels will need this
		// updated to enumerate tenant channels or use a different mechanism.
		jobID, token, _, staticArgs, dynArgs, err := s.workers.RequestJob(
			s.goroutinesCtx, "", []string{worker.JobTypeBootcPreManifest}, []string{""}, uuid.Nil,
		)
		if err != nil {
			<-sem // release on error
			if s.goroutinesCtx.Err() != nil {
				return // server shutting down
			}
			if !errors.Is(err, jobqueue.ErrDequeueTimeout) && !errors.Is(err, jobqueue.ErrNotPending) {
				select {
				case <-time.After(backoff):
				case <-s.goroutinesCtx.Done():
					return
				}
				backoff = min(backoff*2, maxBackoff)
			}
			continue
		}

		backoff = time.Second // reset on success

		s.goroutinesGroup.Add(1)
		go func() {
			defer func() { <-sem }()
			defer s.goroutinesGroup.Done()
			handleBootcPreManifest(s.workers, jobID, token, staticArgs, dynArgs)
		}()
	}
}

// handleBootcPreManifest processes a single BootcPreManifest job. The goal is to
// generate a pre-manifest from resolved bootc container to get the sources for
// downstream resolve jobs.
//
// The job is processed in the following steps:
// 1. Reads resolved bootc container info from dynArgs.
// 2. Creates a bootc distro.
// 3. Generates a pre-manifest.
// 4. Extracts container source specs for downstream resolve jobs.
// 5. Validates that the bootc image type does not unexpectedly require packages or ostree commits.
func handleBootcPreManifest(
	workers *worker.Server,
	jobID uuid.UUID,
	token uuid.UUID,
	staticArgs json.RawMessage,
	dynArgs []json.RawMessage,
) {
	logWithId := logrus.WithField("jobId", jobID)

	var preManifestResult worker.BootcPreManifestJobResult
	defer func() {
		if r := recover(); r != nil {
			logWithId.Errorf("Recovered from panic in handleBootcPreManifest: %v", r)
			preManifestResult.JobError = clienterrors.New(clienterrors.ErrorJobPanicked, "Error extracting sources from pre-manifest", r)
		}

		result, err := json.Marshal(preManifestResult)
		if err != nil {
			logWithId.Errorf("Error marshalling bootc pre-manifest result: %v", err)
		}
		if err := workers.FinishJob(token, result); err != nil {
			logWithId.Errorf("Error finishing bootc pre-manifest job: %v", err)
		}
	}()

	var preManifestArgs worker.BootcPreManifestJob
	if err := json.Unmarshal(staticArgs, &preManifestArgs); err != nil {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorParsingJobArgs,
			"Error parsing bootc pre-manifest job args: "+err.Error(), nil,
		)
		return
	}

	// Validate dynarg index for the bootc info resolve result
	bootcIRArgsIdx := preManifestArgs.BootcInfoResolveDynArgsIdx
	if bootcIRArgsIdx == nil || *bootcIRArgsIdx < 0 || *bootcIRArgsIdx >= len(dynArgs) {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorParsingDynamicArgs,
			"BootcInfoResolveDynArgsIdx is missing or out of range", nil,
		)
		return
	}
	var bootcInfoResult worker.BootcInfoResolveJobResult
	if err := json.Unmarshal(dynArgs[*preManifestArgs.BootcInfoResolveDynArgsIdx], &bootcInfoResult); err != nil {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorParsingDynamicArgs,
			"Error parsing bootc info resolve result: "+err.Error(), nil,
		)
		return
	}
	if bootcInfoResult.JobError != nil {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorJobDependency,
			"Bootc info resolve dependency failed", bootcInfoResult.JobError.Reason,
		)
		return
	}
	// Extract base container info
	if preManifestArgs.BaseInfoIdx < 0 || preManifestArgs.BaseInfoIdx >= len(bootcInfoResult.Infos) {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorParsingDynamicArgs,
			fmt.Sprintf("BaseInfoIdx %d is out of range (resolved %d infos)",
				preManifestArgs.BaseInfoIdx, len(bootcInfoResult.Infos)), nil,
		)
		return
	}
	baseInfo, err := bootcInfoResult.Infos[preManifestArgs.BaseInfoIdx].ToVendor()
	if err != nil {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorManifestGeneration,
			"Error converting base bootc info to vendor type: "+err.Error(), nil,
		)
		return
	}
	// Create bootc distro from base container info
	bootcDistro, err := generic.NewBootc("bootc", baseInfo)
	if err != nil {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorManifestGeneration,
			"Error creating bootc distro: "+err.Error(), nil,
		)
		return
	}
	// Set build container if a separate build info index is specified
	if preManifestArgs.BuildInfoIdx != nil {
		if *preManifestArgs.BuildInfoIdx < 0 || *preManifestArgs.BuildInfoIdx >= len(bootcInfoResult.Infos) {
			preManifestResult.JobError = clienterrors.New(
				clienterrors.ErrorParsingDynamicArgs,
				fmt.Sprintf("BuildInfoIdx %d is out of range (resolved %d infos)",
					*preManifestArgs.BuildInfoIdx, len(bootcInfoResult.Infos)), nil,
			)
			return
		}
		buildInfo, err := bootcInfoResult.Infos[*preManifestArgs.BuildInfoIdx].ToVendor()
		if err != nil {
			preManifestResult.JobError = clienterrors.New(
				clienterrors.ErrorManifestGeneration,
				"Error converting build bootc info to vendor type: "+err.Error(), nil,
			)
			return
		}
		if err := bootcDistro.SetBuildContainer(buildInfo); err != nil {
			preManifestResult.JobError = clienterrors.New(
				clienterrors.ErrorManifestGeneration,
				"Error setting build container: "+err.Error(), nil,
			)
			return
		}
	}

	canonicalArch, err := arch.FromString(baseInfo.Arch)
	if err != nil {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorManifestGeneration,
			fmt.Sprintf("Error parsing the architecture %q from the base bootc info: %s", baseInfo.Arch, err.Error()),
			nil,
		)
		return
	}
	archi, err := bootcDistro.GetArch(canonicalArch.String())
	if err != nil {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorManifestGeneration,
			"Error getting arch from bootc distro: "+err.Error(), nil,
		)
		return
	}
	imgType, err := archi.GetImageType(preManifestArgs.ImageType)
	if err != nil {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorManifestGeneration,
			"Error getting image type from bootc distro: "+err.Error(), nil,
		)
		return
	}

	// Generate pre-manifest.
	// NOTE: Bootc image types ignore the repos parameter (all content comes from containers), so nil is safe.
	manifestSource, _, err := imgType.Manifest(&preManifestArgs.Blueprint, preManifestArgs.ImageOptions, nil, &preManifestArgs.Seed)
	if err != nil {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorManifestGeneration,
			"Error generating bootc pre-manifest: "+err.Error(), nil,
		)
		return
	}

	// Bootc images should not require package depsolving or ostree commits.
	// All content comes from containers. Error if the pre-manifest unexpectedly
	// requests these - it means something is wrong with the image type definition.
	pkgSets, _ := manifestSource.GetPackageSetChains()
	if len(pkgSets) > 0 {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorManifestGeneration,
			"bootc pre-manifest unexpectedly requires package depsolving", nil,
		)
		return
	}
	ostreeSources := manifestSource.GetOSTreeSourceSpecs()
	if len(ostreeSources) > 0 {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorManifestGeneration,
			"bootc pre-manifest unexpectedly requires ostree commit resolution", nil,
		)
		return
	}

	// Extract content sources into result
	// Container sources - build ContainerResolveJob args from manifest content specs
	containerSources := manifestSource.GetContainerSourceSpecs()

	// Bootc image manifests should always return at least one container source.
	if len(containerSources) == 0 {
		preManifestResult.JobError = clienterrors.New(
			clienterrors.ErrorManifestGeneration,
			"bootc pre-manifest unexpectedly didn't return any container sources", nil,
		)
		return
	}

	pipelineContainerSpecs := make(map[string][]worker.ContainerSpec, len(containerSources))
	for name, pipelineSources := range containerSources {
		pipelineContainerSpecs[name] = make([]worker.ContainerSpec, len(pipelineSources))
		for i, source := range pipelineSources {
			pipelineContainerSpecs[name][i] = worker.ContainerSpecFromVendorSourceSpec(source)
		}
	}
	preManifestResult.ContainerResolveJobArgs = &worker.ContainerResolveJob{
		Arch:          canonicalArch.String(),
		PipelineSpecs: pipelineContainerSpecs,
	}
}
