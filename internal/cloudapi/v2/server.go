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
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
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
		PackageSets:      ir.imageType.PackageSets(bp, ir.repositories),
		ModulePlatformID: distribution.ModulePlatformID(),
		Arch:             ir.arch.Name(),
		Releasever:       distribution.Releasever(),
	}, channel)
	if err != nil {
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	manifestJobID, err := s.workers.EnqueueManifestJobByID(&worker.ManifestJobByID{}, depsolveJobID, channel)
	if err != nil {
		return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	id, err = s.workers.EnqueueOSBuildAsDependency(ir.arch.Name(), &worker.OSBuildJob{
		Targets: []*target.Target{ir.target},
		Exports: ir.imageType.Exports(),
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
		generateManifest(s.goroutinesCtx, s.workers, depsolveJobID, manifestJobID, ir.imageType, ir.repositories, ir.imageOptions, manifestSeed, bp.Customizations)
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
			PackageSets:      ir.imageType.PackageSets(bp, ir.repositories),
			ModulePlatformID: distribution.ModulePlatformID(),
			Arch:             ir.arch.Name(),
			Releasever:       distribution.Releasever(),
		}, channel)
		if err != nil {
			return id, HTTPErrorWithInternal(ErrorEnqueueingJob, err)
		}

		manifestJobID, err := s.workers.EnqueueManifestJobByID(&worker.ManifestJobByID{}, depsolveJobID, channel)
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
		buildID, err := s.workers.EnqueueOSBuildAsDependency(ir.arch.Name(), &worker.OSBuildJob{
			ImageName: ir.imageType.Filename(),
			Exports:   ir.imageType.Exports(),
			PipelineNames: &worker.PipelineNames{
				Build:   ir.imageType.BuildPipelines(),
				Payload: ir.imageType.PayloadPipelines(),
			},
			Targets: []*target.Target{target.NewKojiTarget(&target.KojiTargetOptions{
				Server:          server,
				UploadDirectory: kojiDirectory,
				Filename:        kojiFilename,
			})},
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
			generateManifest(s.goroutinesCtx, s.workers, depsolveJobID, manifestJobID, ir.imageType, ir.repositories, ir.imageOptions, manifestSeed, bp.Customizations)
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

func generateManifest(ctx context.Context, workers *worker.Server, depsolveJobID uuid.UUID, manifestJobID uuid.UUID, imageType distro.ImageType, repos []rpmmd.RepoConfig, options distro.ImageOptions, seed int64, b *blueprint.Customizations) {
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
		jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorNoDynamicArgs, reason)
		return
	}

	var depsolveResults worker.DepsolveJobResult
	err = json.Unmarshal(dynArgs[0], &depsolveResults)
	if err != nil {
		reason := "Error parsing dynamic arguments"
		jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorParsingDynamicArgs, reason)
		return
	}

	_, _, err = workers.DepsolveJobStatus(depsolveJobID, &depsolveResults)
	if err != nil {
		reason := "Error reading depsolve status"
		jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorReadingJobStatus, reason)
		return
	}

	if jobErr := depsolveResults.JobError; jobErr != nil {
		if jobErr.ID == clienterrors.ErrorDNFDepsolveError || jobErr.ID == clienterrors.ErrorDNFMarkingErrors {
			jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDepsolveDependency, "Error in depsolve job dependency input, bad package set requested")
			return
		}
		jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDepsolveDependency, "Error in depsolve job dependency")
		return
	}

	if len(depsolveResults.PackageSpecs) == 0 {
		jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorEmptyPackageSpecs, "Received empty package specs")
		return
	}

	manifest, err := imageType.Manifest(b, options, repos, depsolveResults.PackageSpecs, seed)
	if err != nil {
		reason := "Error generating manifest"
		jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorManifestGeneration, reason)
		return
	}

	jobResult.Manifest = manifest
}
