package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type KojiFinalizeJobImpl struct {
	KojiServers map[string]kojiServer
}

func (impl *KojiFinalizeJobImpl) kojiImport(
	server string,
	build koji.ImageBuild,
	buildRoots []koji.BuildRoot,
	images []koji.Image,
	directory, token string) error {

	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	kojiServer, exists := impl.KojiServers[serverURL.Hostname()]
	if !exists {
		return fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	transport := koji.CreateKojiTransport(kojiServer.relaxTimeoutFactor)
	k, err := koji.NewFromGSSAPI(server, &kojiServer.creds, transport)
	if err != nil {
		return err
	}
	defer func() {
		err := k.Logout()
		if err != nil {
			logrus.Warnf("koji logout failed: %v", err)
		}
	}()

	_, err = k.CGImport(build, buildRoots, images, directory, token)
	if err != nil {
		return fmt.Errorf("Could not import build into koji: %v", err)
	}

	return nil
}

func (impl *KojiFinalizeJobImpl) kojiFail(server string, buildID int, token string) error {

	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	kojiServer, exists := impl.KojiServers[serverURL.Hostname()]
	if !exists {
		return fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	transport := koji.CreateKojiTransport(kojiServer.relaxTimeoutFactor)
	k, err := koji.NewFromGSSAPI(server, &kojiServer.creds, transport)
	if err != nil {
		return err
	}
	defer func() {
		err := k.Logout()
		if err != nil {
			logrus.Warnf("koji logout failed: %v", err)
		}
	}()

	return k.CGFailBuild(buildID, token)
}

func (impl *KojiFinalizeJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id().String())

	// initialize the result variable to be used to report status back to composer
	var kojiFinalizeJobResult = &worker.KojiFinalizeJobResult{}
	// initialize / declare variables to be used to report information back to Koji
	var args = &worker.KojiFinalizeJob{}
	var initArgs *worker.KojiInitJobResult

	// In all cases it is necessary to report result back to osbuild-composer worker API.
	defer func() {
		err := job.Update(kojiFinalizeJobResult)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}

		// Fail the Koji build if the job error is set and the necessary
		// information to identify the job are available.
		if kojiFinalizeJobResult.JobError != nil && initArgs != nil {
			err = impl.kojiFail(args.Server, int(initArgs.BuildID), initArgs.Token)
			if err != nil {
				logWithId.Errorf("Failing Koji job failed: %v", err)
			}
		}
	}()

	err := job.Args(args)
	if err != nil {
		kojiFinalizeJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorParsingJobArgs, "Error parsing job args", err.Error())
		return err
	}

	build := koji.ImageBuild{
		TaskID:    args.TaskID,
		Name:      args.Name,
		Version:   args.Version,
		Release:   args.Release,
		StartTime: int64(args.StartTime),
		EndTime:   time.Now().Unix(),
	}

	var buildRoots []koji.BuildRoot
	var images []koji.Image

	var osbuildResults []worker.OSBuildJobResult
	initArgs, osbuildResults, err = extractDynamicArgs(job)
	if err != nil {
		kojiFinalizeJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorParsingDynamicArgs, "Error parsing dynamic args", err.Error())
		return err
	}
	build.BuildID = initArgs.BuildID

	// Check the dependencies early.
	if hasFailedDependency(*initArgs, osbuildResults) {
		kojiFinalizeJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorKojiFailedDependency, "At least one job dependency failed", nil)
		return nil
	}

	for i, buildArgs := range osbuildResults {
		buildRPMs := make([]rpmmd.RPM, 0)
		// collect packages from stages in build pipelines
		for _, plName := range buildArgs.PipelineNames.Build {
			buildPipelineMd := buildArgs.OSBuildOutput.Metadata[plName]
			buildRPMs = append(buildRPMs, osbuild.OSBuildMetadataToRPMs(buildPipelineMd)...)
		}
		// this dedupe is usually not necessary since we generally only have
		// one rpm stage in one build pipeline, but it's not invalid to have
		// multiple
		buildRPMs = rpmmd.DeduplicateRPMs(buildRPMs)

		kojiTargetResults := buildArgs.TargetResultsByName(target.TargetNameKoji)
		// Only a single Koji target is allowed per osbuild job
		if len(kojiTargetResults) != 1 {
			kojiFinalizeJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorKojiFinalize, "Exactly one Koji target result is expected per osbuild job", nil)
			return nil
		}

		kojiTargetResult := kojiTargetResults[0]
		kojiTargetOptions := kojiTargetResult.Options.(*target.KojiTargetResultOptions)

		buildRoots = append(buildRoots, koji.BuildRoot{
			ID: uint64(i),
			Host: koji.Host{
				Os:   buildArgs.HostOS,
				Arch: buildArgs.Arch,
			},
			ContentGenerator: koji.ContentGenerator{
				Name:    "osbuild",
				Version: "0", // TODO: put the correct version here
			},
			Container: koji.Container{
				Type: "none",
				Arch: buildArgs.Arch,
			},
			Tools: []koji.Tool{},
			RPMs:  buildRPMs,
		})

		// collect packages from stages in payload pipelines
		imageRPMs := make([]rpmmd.RPM, 0)
		for _, plName := range buildArgs.PipelineNames.Payload {
			payloadPipelineMd := buildArgs.OSBuildOutput.Metadata[plName]
			imageRPMs = append(imageRPMs, osbuild.OSBuildMetadataToRPMs(payloadPipelineMd)...)
		}

		// deduplicate
		imageRPMs = rpmmd.DeduplicateRPMs(imageRPMs)

		images = append(images, koji.Image{
			BuildRootID:  uint64(i),
			Filename:     args.KojiFilenames[i],
			FileSize:     kojiTargetOptions.ImageSize,
			Arch:         buildArgs.Arch,
			ChecksumType: "md5",
			MD5:          kojiTargetOptions.ImageMD5,
			Type:         "image",
			RPMs:         imageRPMs,
			Extra: koji.ImageExtra{
				Info: koji.ImageExtraInfo{
					Arch: buildArgs.Arch,
				},
			},
		})
	}

	err = impl.kojiImport(args.Server, build, buildRoots, images, args.KojiDirectory, initArgs.Token)
	if err != nil {
		kojiFinalizeJobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorKojiFinalize, err.Error(), nil)
		return err
	}

	return nil
}

// Extracts dynamic args of the koji-finalize job. Returns an error if they
// cannot be unmarshalled.
func extractDynamicArgs(job worker.Job) (*worker.KojiInitJobResult, []worker.OSBuildJobResult, error) {
	var kojiInitResult worker.KojiInitJobResult
	err := job.DynamicArgs(0, &kojiInitResult)
	if err != nil {
		return nil, nil, err
	}

	osbuildResults := make([]worker.OSBuildJobResult, job.NDynamicArgs()-1)

	for i := 1; i < job.NDynamicArgs(); i++ {
		err = job.DynamicArgs(i, &osbuildResults[i-1])
		if err != nil {
			return nil, nil, err
		}
	}

	return &kojiInitResult, osbuildResults, nil
}

// Returns true if any of koji-finalize dependencies failed.
func hasFailedDependency(kojiInitResult worker.KojiInitJobResult, osbuildResults []worker.OSBuildJobResult) bool {
	if kojiInitResult.JobError != nil {
		return true
	}

	for _, r := range osbuildResults {
		// No `OSBuildOutput` implies failure: either osbuild crashed or
		// rejected the input (manifest or command line arguments)
		if r.OSBuildOutput == nil || !r.OSBuildOutput.Success || r.JobError != nil {
			return true
		}
	}
	return false
}
