package main

import (
	"fmt"
	"os/exec"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/bootc"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type resolveBootcInfoFuncType func(ref string) (*bootc.Info, error)
type removeContainerImageFuncType func(ref string) error

// variables to allow for testing
var resolveBootcInfoFunc resolveBootcInfoFuncType = bootc.ResolveBootcInfo
var resolveBootcBuildInfoFunc resolveBootcInfoFuncType = bootc.ResolveBootcBuildInfo
var removeContainerImageFunc removeContainerImageFuncType = removeContainerImage

func removeContainerImage(ref string) error {
	cmd := exec.Command("podman", "rmi", ref)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman rmi %q: %s: %w", ref, string(output), err)
	}
	return nil
}

type BootcInfoResolveJobImpl struct {
	CleanupImages bool
}

func (impl *BootcInfoResolveJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id())

	result := worker.BootcInfoResolveJobResult{}
	defer func() {
		if r := recover(); r != nil {
			logWithId.Errorf("Recovered from panic in BootcInfoResolveJobImpl.Run: %v", r)
			result.JobError = clienterrors.New(clienterrors.ErrorJobPanicked, "Error resolving bootc info", r)
		}

		err := job.Finish(&result)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}
	}()

	var args worker.BootcInfoResolveJob
	var err error
	if err = job.Args(&args); err != nil {
		result.JobError = clienterrors.New(clienterrors.ErrorParsingJobArgs, "Error parsing bootc info resolve job args: "+err.Error(), nil)
		return fmt.Errorf("Error parsing bootc info resolve job args: %v", err)
	}

	resolvedInfos := make([]worker.BootcContainerInfo, 0, len(args.Specs))
	for _, spec := range args.Specs {
		logWithId.Infof("Resolving bootc container info (ref: %s, resolve_mode: %s)", spec.Ref, spec.ResolveMode)

		var info *bootc.Info
		if spec.ResolveMode == worker.BootcInfoResolveModeFull {
			// Full resolution for the base container:
			// ResolveBootcInfo handles container lifecycle (start + stop)
			info, err = resolveBootcInfoFunc(spec.Ref)
		} else if spec.ResolveMode == worker.BootcInfoResolveModeBuild {
			// Minimal resolution for the build container:
			// ResolveBootcBuildInfo handles container lifecycle (start + stop)
			info, err = resolveBootcBuildInfoFunc(spec.Ref)
		} else {
			// NOTE: This should never happen, because the UnmarshalJSON method
			// should have validated the resolve mode. But let's be defensive.
			reason := fmt.Sprintf("invalid resolve mode: %s", spec.ResolveMode)
			result.JobError = clienterrors.New(clienterrors.ErrorBootcInfoResolve, reason, nil)
			return fmt.Errorf("invalid resolve mode: %s", spec.ResolveMode)
		}

		if err != nil {
			reason := fmt.Sprintf("failed to resolve bootc info for ref %q: %s", spec.Ref, err.Error())
			result.JobError = clienterrors.New(clienterrors.ErrorBootcInfoResolve, reason, err)
			return fmt.Errorf("failed to resolve bootc info for ref %q: %s", spec.Ref, err.Error())
		}

		if impl.CleanupImages {
			ref := spec.Ref
			defer func() {
				if err := removeContainerImageFunc(ref); err != nil {
					logWithId.Warnf("Failed to cleanup container image %q: %v", ref, err)
				}
			}()
		}

		// Convert vendor type to DTO for the job result
		resolvedInfo, err := worker.BootcContainerInfoFromVendor(info)
		if err != nil {
			reason := fmt.Sprintf("failed to convert bootc info to DTO for ref %q: %s", spec.Ref, err.Error())
			result.JobError = clienterrors.New(clienterrors.ErrorBootcInfoResolve, reason, nil)
			return fmt.Errorf("failed to convert bootc info to DTO for ref %q: %s", spec.Ref, err.Error())
		}
		resolvedInfos = append(resolvedInfos, *resolvedInfo)
	}

	result.Infos = resolvedInfos

	return nil
}
