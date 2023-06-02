package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/prometheus"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

// Server represents the state of the cloud Server
type Server struct {
	workers *worker.Server
	distros *distroregistry.Registry
	config  ServerConfig
	router  routers.Router

	goroutinesCtx       context.Context
	goroutinesCtxCancel context.CancelFunc
	goroutinesGroup     sync.WaitGroup
}

type ServerConfig struct {
	TenantProviderFields []string
	JWTEnabled           bool
}

func NewServer(workers *worker.Server, distros *distroregistry.Registry, config ServerConfig) *Server {
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
	e.HTTPErrorHandler = s.HTTPErrorHandler
	e.Pre(common.OperationIDMiddleware)
	e.Use(middleware.Recover())
	e.Logger = common.Logger()

	handler := apiHandlers{
		server: s,
	}

	statusMW := prometheus.StatusMiddleware(prometheus.ComposerSubsystem)
	RegisterHandlers(e.Group(path, prometheus.MetricsMiddleware, s.ValidateRequest, statusMW), &handler)

	return e
}

func (s *Server) Shutdown() {
	s.goroutinesCtxCancel()
	s.goroutinesGroup.Wait()
}

func (s *Server) enqueueCompose(distribution distro.Distro, bp blueprint.Blueprint, manifestSeed int64, irs []imageRequest, channel string) (uuid.UUID, error) {
	var id uuid.UUID
	if len(irs) != 1 {
		return id, HTTPError(ErrorInvalidNumberOfImageBuilds)
	}
	ir := irs[0]

	manifestSource, _, err := ir.imageType.Manifest(&bp, ir.imageOptions, ir.repositories, manifestSeed)
	if err != nil {
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	depsolveJobID, err := s.workers.EnqueueDepsolve(&worker.DepsolveJob{
		PackageSets:      manifestSource.GetPackageSetChains(),
		ModulePlatformID: distribution.ModulePlatformID(),
		Arch:             ir.arch.Name(),
		Releasever:       distribution.Releasever(),
	}, channel)
	if err != nil {
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}
	dependencies := []uuid.UUID{depsolveJobID}

	var containerResolveJobID uuid.UUID
	containerSources := manifestSource.GetContainerSourceSpecs()
	if len(containerSources) > 1 {
		// only one pipeline can embed containers
		pipelines := make([]string, 0, len(containerSources))
		for name := range containerSources {
			pipelines = append(pipelines, name)
		}
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, fmt.Errorf("manifest returned %d pipelines with containers (at most 1 is supported): %s", len(containerSources), strings.Join(pipelines, ", ")))
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
			Arch:  ir.arch.Name(),
			Specs: workerResolveSpecs,
		}

		jobId, err := s.workers.EnqueueContainerResolveJob(&job, channel)
		if err != nil {
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}

		containerResolveJobID = jobId
		dependencies = append(dependencies, containerResolveJobID)
		break // there can be only one
	}

	var ostreeResolveJobID uuid.UUID
	commitSources := manifestSource.GetOSTreeSourceSpecs()
	if len(commitSources) > 1 {
		// only one pipeline can specify an ostree commit for content
		pipelines := make([]string, 0, len(commitSources))
		for name := range commitSources {
			pipelines = append(pipelines, name)
		}
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, fmt.Errorf("manifest returned %d pipelines with ostree commits (at most 1 is supported): %s", len(commitSources), strings.Join(pipelines, ", ")))
	}
	for _, sources := range commitSources {
		workerResolveSpecs := make([]worker.OSTreeResolveSpec, len(sources))
		for idx, source := range sources {
			// ostree.SourceSpec is directly convertible to worker.OSTreeResolveSpec
			workerResolveSpecs[idx] = worker.OSTreeResolveSpec(source)
		}
		jobID, err := s.workers.EnqueueOSTreeResolveJob(&worker.OSTreeResolveJob{Specs: workerResolveSpecs}, channel)
		if err != nil {
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}

		ostreeResolveJobID = jobID
		dependencies = append(dependencies, ostreeResolveJobID)
		break // there can be only one
	}

	manifestJobID, err := s.workers.EnqueueManifestJobByID(&worker.ManifestJobByID{}, dependencies, channel)
	if err != nil {
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	id, err = s.workers.EnqueueOSBuildAsDependency(ir.arch.Name(), &worker.OSBuildJob{
		Targets: []*target.Target{ir.target},
		PipelineNames: &worker.PipelineNames{
			Build:   ir.imageType.BuildPipelines(),
			Payload: ir.imageType.PayloadPipelines(),
		},
	}, []uuid.UUID{manifestJobID}, channel)
	if err != nil {
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	s.goroutinesGroup.Add(1)
	go func() {
		serializeManifest(s.goroutinesCtx, manifestSource, s.workers, depsolveJobID, containerResolveJobID, ostreeResolveJobID, manifestJobID, manifestSeed)
		defer s.goroutinesGroup.Done()
	}()

	return id, nil
}

func (s *Server) enqueueKojiCompose(taskID uint64, server, name, version, release string, distribution distro.Distro, bp blueprint.Blueprint, manifestSeed int64, irs []imageRequest, channel string) (uuid.UUID, error) {
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
	for _, ir := range irs {
		manifestSource, _, err := ir.imageType.Manifest(&bp, ir.imageOptions, ir.repositories, manifestSeed)
		if err != nil {
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}

		depsolveJobID, err := s.workers.EnqueueDepsolve(&worker.DepsolveJob{
			PackageSets:      manifestSource.GetPackageSetChains(),
			ModulePlatformID: distribution.ModulePlatformID(),
			Arch:             ir.arch.Name(),
			Releasever:       distribution.Releasever(),
		}, channel)
		if err != nil {
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}
		dependencies := []uuid.UUID{depsolveJobID}

		var containerResolveJobID uuid.UUID
		containerSources := manifestSource.GetContainerSourceSpecs()
		if len(containerSources) > 1 {
			// only one pipeline can embed containers
			pipelines := make([]string, 0, len(containerSources))
			for name := range containerSources {
				pipelines = append(pipelines, name)
			}
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, fmt.Errorf("manifest returned %d pipelines with containers (at most 1 is supported): %s", len(containerSources), strings.Join(pipelines, ", ")))
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
				Arch:  ir.arch.Name(),
				Specs: make([]worker.ContainerSpec, len(bp.Containers)),
			}

			jobId, err := s.workers.EnqueueContainerResolveJob(&job, channel)
			if err != nil {
				return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
			}

			containerResolveJobID = jobId
			dependencies = append(dependencies, containerResolveJobID)
			break // there can be only one
		}

		var ostreeResolveJobID uuid.UUID
		commitSources := manifestSource.GetOSTreeSourceSpecs()
		if len(commitSources) > 1 {
			// only one pipeline can specify an ostree commit for content
			pipelines := make([]string, 0, len(commitSources))
			for name := range commitSources {
				pipelines = append(pipelines, name)
			}
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, fmt.Errorf("manifest returned %d pipelines with ostree commits (at most 1 is supported): %s", len(commitSources), strings.Join(pipelines, ", ")))
		}
		for _, sources := range commitSources {
			workerResolveSpecs := make([]worker.OSTreeResolveSpec, len(sources))
			for idx, source := range sources {
				// ostree.SourceSpec is directly convertible to worker.OSTreeResolveSpec
				workerResolveSpecs[idx] = worker.OSTreeResolveSpec(source)
			}
			jobID, err := s.workers.EnqueueOSTreeResolveJob(&worker.OSTreeResolveJob{Specs: workerResolveSpecs}, channel)
			if err != nil {
				return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
			}

			ostreeResolveJobID = jobID
			dependencies = append(dependencies, ostreeResolveJobID)
			break // there can be only one
		}

		manifestJobID, err := s.workers.EnqueueManifestJobByID(&worker.ManifestJobByID{}, dependencies, channel)
		if err != nil {
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}
		kojiFilename := fmt.Sprintf(
			"%s-%s-%s.%s%s",
			name,
			version,
			release,
			ir.arch.Name(),
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
		// add any cloud upload target if defined
		if ir.target != nil {
			targets = append(targets, ir.target)
		}

		buildID, err := s.workers.EnqueueOSBuildAsDependency(ir.arch.Name(), &worker.OSBuildJob{
			PipelineNames: &worker.PipelineNames{
				Build:   ir.imageType.BuildPipelines(),
				Payload: ir.imageType.PayloadPipelines(),
			},
			Targets:            targets,
			ManifestDynArgsIdx: common.ToPtr(1),
		}, []uuid.UUID{initID, manifestJobID}, channel)
		if err != nil {
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}
		kojiFilenames = append(kojiFilenames, kojiFilename)
		buildIDs = append(buildIDs, buildID)

		// copy the image request while passing it into the goroutine to prevent data races
		s.goroutinesGroup.Add(1)
		go func(ir imageRequest) {
			serializeManifest(s.goroutinesCtx, manifestSource, s.workers, depsolveJobID, containerResolveJobID, ostreeResolveJobID, manifestJobID, manifestSeed)
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
		StartTime:     uint64(time.Now().Unix()),
	}, initID, buildIDs, channel)
	if err != nil {
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	return id, nil
}

func serializeManifest(ctx context.Context, manifestSource *manifest.Manifest, workers *worker.Server, depsolveJobID, containerResolveJobID, ostreeResolveJobID, manifestJobID uuid.UUID, seed int64) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()

	// wait until job is in a pending state
	var token uuid.UUID
	var dynArgs []json.RawMessage
	var err error
	logWithId := logrus.WithField("jobId", manifestJobID)
	for {
		_, token, _, _, dynArgs, err = workers.RequestJobById(ctx, "", manifestJobID)
		if err == jobqueue.ErrNotPending {
			logWithId.Debug("Manifest job not pending, waiting for depsolve job to finish")
			time.Sleep(time.Millisecond * 50)
			select {
			case <-ctx.Done():
				logWithId.Warning("Manifest job dependencies took longer than 5 minutes to finish, or the server is shutting down, returning to avoid dangling routines")
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

	jobResult := &worker.ManifestJobByIDResult{
		Manifest: nil,
	}

	defer func() {
		if jobResult.JobError != nil {
			logWithId.Errorf("Error in manifest job %v: %v", jobResult.JobError.Reason, err)
		}

		result, err := json.Marshal(jobResult)
		if err != nil {
			logWithId.Errorf("Error marshalling manifest job results: %v", err)
		}

		err = workers.FinishJob(token, result)
		if err != nil {
			logWithId.Errorf("Error finishing manifest job: %v", err)
		}
	}()

	if len(dynArgs) == 0 {
		reason := "No dynamic arguments"
		jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorNoDynamicArgs, reason, nil)
		return
	}

	var depsolveResults worker.DepsolveJobResult
	err = json.Unmarshal(dynArgs[0], &depsolveResults)
	if err != nil {
		reason := "Error parsing dynamic arguments"
		jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorParsingDynamicArgs, reason, nil)
		return
	}

	_, err = workers.DepsolveJobInfo(depsolveJobID, &depsolveResults)
	if err != nil {
		reason := "Error reading depsolve status"
		jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorReadingJobStatus, reason, nil)
		return
	}

	if jobErr := depsolveResults.JobError; jobErr != nil {
		if jobErr.ID == clienterrors.ErrorDNFDepsolveError || jobErr.ID == clienterrors.ErrorDNFMarkingErrors {
			jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDepsolveDependency, "Error in depsolve job dependency input, bad package set requested", nil)
			return
		}
		jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDepsolveDependency, "Error in depsolve job dependency", nil)
		return
	}

	if len(depsolveResults.PackageSpecs) == 0 {
		jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorEmptyPackageSpecs, "Received empty package specs", nil)
		return
	}

	var containerSpecs map[string][]container.Spec
	if containerResolveJobID != uuid.Nil {
		// Container resolve job
		var result worker.ContainerResolveJobResult
		_, err := workers.ContainerResolveJobInfo(containerResolveJobID, &result)

		if err != nil {
			reason := "Error reading container resolve job status"
			jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorReadingJobStatus, reason, nil)
			return
		}

		if jobErr := result.JobError; jobErr != nil {
			jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorContainerDependency, "Error in container resolve job dependency", nil)
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
	if ostreeResolveJobID != uuid.Nil {
		var result worker.OSTreeResolveJobResult
		_, err := workers.OSTreeResolveJobInfo(ostreeResolveJobID, &result)

		if err != nil {
			reason := "Error reading ostree resolve job status"
			logrus.Errorf("%s: %v", reason, err)
			jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorReadingJobStatus, reason, nil)
			return
		}

		if jobErr := result.JobError; jobErr != nil {
			jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorOSTreeDependency, "Error in ostree resolve job dependency", nil)
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
			}
		}
		ostreeCommitSpecs = map[string][]ostree.CommitSpec{
			ostreeCommitPipeline: commitSpecs,
		}
	}

	ms, err := manifestSource.Serialize(depsolveResults.PackageSpecs, containerSpecs, ostreeCommitSpecs)

	jobResult.Manifest = ms
}
