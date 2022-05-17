package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"

	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type KojiFinalizeJobImpl struct {
	KojiServers        map[string]koji.GSSAPICredentials
	relaxTimeoutFactor uint
}

func (impl *KojiFinalizeJobImpl) kojiImport(
	server string,
	build koji.ImageBuild,
	buildRoots []koji.BuildRoot,
	images []koji.Image,
	directory, token string) error {
	transport := koji.CreateKojiTransport(impl.relaxTimeoutFactor)

	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	creds, exists := impl.KojiServers[serverURL.Hostname()]
	if !exists {
		return fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	k, err := koji.NewFromGSSAPI(server, &creds, transport)
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
	transport := koji.CreateKojiTransport(impl.relaxTimeoutFactor)

	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	creds, exists := impl.KojiServers[serverURL.Hostname()]
	if !exists {
		return fmt.Errorf("Koji server has not been configured: %s", serverURL.Hostname())
	}

	k, err := koji.NewFromGSSAPI(server, &creds, transport)
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
	var args worker.KojiFinalizeJob
	err := job.Args(&args)
	if err != nil {
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

	var initArgs *worker.KojiInitJobResult
	var buildRoots []koji.BuildRoot
	var images []koji.Image

	isOldKojiCompose, err := isOldKojiComposeReq(job)
	if err != nil {
		return err
	}

	// TODO: remove eventually. Kept for backward compatibility.
	if isOldKojiCompose {
		var osbuildKojiResults []worker.OSBuildKojiJobResult
		initArgs, osbuildKojiResults, err = extractDynamicArgsOld(job)
		if err != nil {
			return err
		}
		build.BuildID = initArgs.BuildID

		// Check the dependencies early. Fail the koji build if any of them failed.
		if hasFailedDependencyOld(*initArgs, osbuildKojiResults) {
			err = impl.kojiFail(args.Server, int(initArgs.BuildID), initArgs.Token)

			// Update the status immediately and bail out.
			var result worker.KojiFinalizeJobResult
			if err != nil {
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorKojiFailedDependency, err.Error())
			}
			err = job.Update(&result)
			if err != nil {
				return fmt.Errorf("Error reporting job result: %v", err)
			}
			return nil
		}

		for i, buildArgs := range osbuildKojiResults {
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
				FileSize:     buildArgs.ImageSize,
				Arch:         buildArgs.Arch,
				ChecksumType: "md5",
				MD5:          buildArgs.ImageHash,
				Type:         "image",
				RPMs:         imageRPMs,
				Extra: koji.ImageExtra{
					Info: koji.ImageExtraInfo{
						Arch: buildArgs.Arch,
					},
				},
			})
		}
	} else {
		var osbuildResults []worker.OSBuildJobResult
		initArgs, osbuildResults, err = extractDynamicArgs(job)
		if err != nil {
			return err
		}
		build.BuildID = initArgs.BuildID

		// Check the dependencies early. Fail the koji build if any of them failed.
		if hasFailedDependency(*initArgs, osbuildResults) {
			err = impl.kojiFail(args.Server, int(initArgs.BuildID), initArgs.Token)

			// Update the status immediately and bail out.
			var result worker.KojiFinalizeJobResult
			if err != nil {
				result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorKojiFailedDependency, err.Error())
			}
			err = job.Update(&result)
			if err != nil {
				return fmt.Errorf("Error reporting job result: %v", err)
			}
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

			// TODO: support multiple upload targets
			if len(buildArgs.TargetResults) != 1 {
				// TODO: should we call kojiFail() and update job status, instead of just returning?
				return fmt.Errorf("error: Koji compose OSBuild job result doesn't contain exactly one target result")
			}
			kojiTarget := buildArgs.TargetResults[0]
			kojiTargetOptions := kojiTarget.Options.(target.KojiTargetResultOptions)

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
	}

	var result worker.KojiFinalizeJobResult
	err = impl.kojiImport(args.Server, build, buildRoots, images, args.KojiDirectory, initArgs.Token)
	if err != nil {
		result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorKojiFinalize, err.Error())
	}

	err = job.Update(&result)
	if err != nil {
		return fmt.Errorf("Error reporting job result: %v", err)
	}

	return nil
}

// Extracts dynamic args of the koji-finalize job. Returns an error if they
// cannot be unmarshalled.
func extractDynamicArgsOld(job worker.Job) (*worker.KojiInitJobResult, []worker.OSBuildKojiJobResult, error) {
	var kojiInitResult worker.KojiInitJobResult
	err := job.DynamicArgs(0, &kojiInitResult)
	if err != nil {
		return nil, nil, err
	}

	osbuildKojiResults := make([]worker.OSBuildKojiJobResult, job.NDynamicArgs()-1)

	for i := 1; i < job.NDynamicArgs(); i++ {
		err = job.DynamicArgs(i, &osbuildKojiResults[i-1])
		if err != nil {
			return nil, nil, err
		}
	}

	return &kojiInitResult, osbuildKojiResults, nil
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

// Tests the first osbuild job result for specific values and decides
// if it is a `worker.OSBuildKojiJobResult` or `worker.OSBuildJobResult`.
// In case of `worker.OSBuildKojiJobResult` returns `true`.
// In case of 'worker.OSBuildJobResult` returns `false`.
func isOldKojiComposeReq(job worker.Job) (bool, error) {
	if job.NDynamicArgs() < 2 {
		return false, fmt.Errorf("error: koji-finalize job does not have any osbuild job dynamic arg attached")
	}

	var osbuildKojiResult worker.OSBuildKojiJobResult
	err := job.DynamicArgs(1, &osbuildKojiResult)
	if err != nil {
		return false, err
	}

	// The decision is based on the fact if all result values specific to
	// `worker.OSBuildKojiJobResult` are set to meaningful values or not.
	// The assumption is that the default type values are not meaningful.
	if osbuildKojiResult.Arch == "" || osbuildKojiResult.HostOS == "" ||
		osbuildKojiResult.ImageHash == "" || osbuildKojiResult.ImageSize == 0 {
		return false, nil
	}

	return true, nil
}

// Returns true if any of koji-finalize dependencies failed.
func hasFailedDependencyOld(kojiInitResult worker.KojiInitJobResult, osbuildKojiResults []worker.OSBuildKojiJobResult) bool {
	if kojiInitResult.JobError != nil {
		return true
	}

	for _, r := range osbuildKojiResults {
		// No `OSBuildOutput` implies failure: either osbuild crashed or
		// rejected the input (manifest or command line arguments)
		if r.OSBuildOutput == nil || !r.OSBuildOutput.Success || r.JobError != nil {
			return true
		}
	}
	return false
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
