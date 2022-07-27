package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/osbuild/osbuild-composer/pkg/jobqueue"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/prometheus"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
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
	RegisterHandlers(e.Group(path, prometheus.MetricsMiddleware, s.ValidateRequest), &handler)

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

	depsolveJobID, err := s.workers.EnqueueDepsolve(&worker.DepsolveJob{
		PackageSets:      ir.imageType.PackageSets(bp, ir.imageOptions, ir.repositories),
		ModulePlatformID: distribution.ModulePlatformID(),
		Arch:             ir.arch.Name(),
		Releasever:       distribution.Releasever(),
	}, channel)
	if err != nil {
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	dependencies := []uuid.UUID{depsolveJobID}
	var containerResolveJob uuid.UUID
	if len(bp.Containers) > 0 {
		job := worker.ContainerResolveJob{
			Arch:  ir.arch.Name(),
			Specs: make([]worker.ContainerSpec, len(bp.Containers)),
		}

		for i, c := range bp.Containers {
			job.Specs[i] = worker.ContainerSpec{
				Source:    c.Source,
				Name:      c.Name,
				TLSVerify: c.TLSVerify,
			}
		}

		jobId, err := s.workers.EnqueueContainerResolveJob(&job, channel)

		if err != nil {
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}

		containerResolveJob = jobId
		dependencies = append(dependencies, jobId)
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
		generateManifest(s.goroutinesCtx, s.workers, depsolveJobID, containerResolveJob, manifestJobID, ir.imageType, ir.repositories, ir.imageOptions, manifestSeed, bp.Customizations)
		defer s.goroutinesGroup.Done()
	}()

	return id, nil
}

func (s *Server) enqueueKojiCompose(taskID uint64, server, name, version, release string, distribution distro.Distro, bp blueprint.Blueprint, manifestSeed int64, irs []imageRequest, channel string) (uuid.UUID, error) {
	var id uuid.UUID
	kojiDirectory := "osbuild-composer-koji-" + uuid.New().String()

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
		depsolveJobID, err := s.workers.EnqueueDepsolve(&worker.DepsolveJob{
			PackageSets:      ir.imageType.PackageSets(bp, ir.imageOptions, ir.repositories),
			ModulePlatformID: distribution.ModulePlatformID(),
			Arch:             ir.arch.Name(),
			Releasever:       distribution.Releasever(),
		}, channel)
		if err != nil {
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}

		var containerResolveJob uuid.UUID
		dependencies := []uuid.UUID{depsolveJobID}
		if len(bp.Containers) > 0 {
			job := worker.ContainerResolveJob{
				Arch:  ir.arch.Name(),
				Specs: make([]worker.ContainerSpec, len(bp.Containers)),
			}

			for i, c := range bp.Containers {
				job.Specs[i] = worker.ContainerSpec{
					Source:    c.Source,
					Name:      c.Name,
					TLSVerify: c.TLSVerify,
				}
			}

			jobId, err := s.workers.EnqueueContainerResolveJob(&job, channel)

			if err != nil {
				return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
			}

			containerResolveJob = jobId
			dependencies = append(dependencies, jobId)
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
			ManifestDynArgsIdx: common.IntToPtr(1),
		}, []uuid.UUID{initID, manifestJobID}, channel)
		if err != nil {
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}
		kojiFilenames = append(kojiFilenames, kojiFilename)
		buildIDs = append(buildIDs, buildID)

		// copy the image request while passing it into the goroutine to prevent data races
		s.goroutinesGroup.Add(1)
		go func(ir imageRequest) {
			generateManifest(s.goroutinesCtx, s.workers, depsolveJobID, containerResolveJob, manifestJobID, ir.imageType, ir.repositories, ir.imageOptions, manifestSeed, bp.Customizations)
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

func generateManifest(ctx context.Context, workers *worker.Server, depsolveJobID, containerResolveJobID, manifestJobID uuid.UUID, imageType distro.ImageType, repos []rpmmd.RepoConfig, options distro.ImageOptions, seed int64, b *blueprint.Customizations) {
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

	var containerSpecs []container.Spec
	if len(dynArgs) == 2 {
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

		containerSpecs = make([]container.Spec, len(result.Specs))

		for i, s := range result.Specs {
			containerSpecs[i].Source = s.Source
			containerSpecs[i].Digest = s.Digest
			containerSpecs[i].LocalName = s.Name
			containerSpecs[i].TLSVerify = s.TLSVerify
			containerSpecs[i].ImageID = s.ImageID
		}
	}

	manifest, err := imageType.Manifest(b, options, repos, depsolveResults.PackageSpecs, containerSpecs, seed)
	if err != nil {
		reason := "Error generating manifest"
		jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorManifestGeneration, reason, nil)
		return
	}

	jobResult.Manifest = manifest
}
