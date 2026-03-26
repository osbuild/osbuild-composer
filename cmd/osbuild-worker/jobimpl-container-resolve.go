package main

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type ContainerResolveJobImpl struct {
	AuthFilePath string
}

func (impl *ContainerResolveJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id())

	result := worker.ContainerResolveJobResult{}
	defer func() {
		if r := recover(); r != nil {
			logWithId.Errorf("Recovered from panic in ContainerResolveJobImpl.Run: %v", r)
			result.JobError = clienterrors.New(clienterrors.ErrorJobPanicked, "Error resolving containers", r)
		}

		err := job.Finish(&result)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}
	}()

	var args worker.ContainerResolveJob
	if err := job.Args(&args); err != nil {
		result.JobError = clienterrors.New(
			clienterrors.ErrorParsingJobArgs, "Error parsing container resolve job args: "+err.Error(), nil)
		return fmt.Errorf("Error parsing container resolve job args: %v", err)
	}

	if len(args.PipelineSpecs) > 0 && args.PreManifestDynArgsIdx != nil {
		result.JobError = clienterrors.New(
			clienterrors.ErrorParsingJobArgs, "PipelineSpecs and PreManifestDynArgsIdx cannot be set at the same time", nil)
		return fmt.Errorf("PipelineSpecs and PreManifestDynArgsIdx cannot be set at the same time")
	}

	// If static args have no PipelineSpecs and a PreManifestDynArgsIdx is set, read args from the BootcPreManifest dependency result.
	if len(args.PipelineSpecs) == 0 && args.PreManifestDynArgsIdx != nil {
		dynArgsResult, dynArgsErr := readContainerResolveArgsFromDynArgs(job, *args.PreManifestDynArgsIdx)
		if dynArgsErr != nil {
			result.JobError = dynArgsErr
			return fmt.Errorf("Error reading container resolve args from dynamic args: %v", dynArgsErr)
		}
		if dynArgsResult != nil {
			args = *dynArgsResult
		}
	}

	// No-op: no specs to resolve
	if len(args.PipelineSpecs) == 0 {
		return nil
	}

	totalSpecs := 0
	for _, specs := range args.PipelineSpecs {
		totalSpecs += len(specs)
	}
	logWithId.Infof("Resolving containers (%d specs across %d pipelines)", totalSpecs, len(args.PipelineSpecs))

	// NOTE: container.Resolver.Finish() sorts results by digest, destroying Add() ordering.
	// Use a separate resolver per pipeline because, positional slicing
	// across pipelines would not work, due to the sorting by digest.
	result.PipelineSpecs = make(map[string][]worker.ContainerSpec, len(args.PipelineSpecs))
	for name, specs := range args.PipelineSpecs {
		if len(specs) == 0 {
			continue
		}

		resolver := container.NewResolver(args.Arch)
		resolver.AuthFilePath = impl.AuthFilePath

		for _, s := range specs {
			resolver.Add(s.ToVendorSourceSpec())
		}

		resolved, err := resolver.Finish()
		if err != nil {
			result.JobError = clienterrors.New(clienterrors.ErrorContainerResolution, err.Error(), nil)
			return fmt.Errorf("Error resolving containers for pipeline %q: %v", name, err)
		}

		pipelineResult := make([]worker.ContainerSpec, len(resolved))
		for i, spec := range resolved {
			pipelineResult[i] = worker.ContainerSpecFromVendorSpec(spec)
		}
		result.PipelineSpecs[name] = pipelineResult
	}

	return nil
}

// readContainerResolveArgsFromDynArgs reads the container resolve args from
// a BootcPreManifestJobResult stored in dynamic args at the given index.
func readContainerResolveArgsFromDynArgs(job worker.Job, dynArgsIdx int) (*worker.ContainerResolveJob, *clienterrors.Error) {
	if dynArgsIdx < 0 || dynArgsIdx >= job.NDynamicArgs() {
		return nil, clienterrors.New(
			clienterrors.ErrorParsingDynamicArgs,
			fmt.Sprintf("PreManifestDynArgsIdx %d is out of range", dynArgsIdx),
			nil,
		)
	}

	var preManifestResult worker.BootcPreManifestJobResult
	if err := job.DynamicArgs(dynArgsIdx, &preManifestResult); err != nil {
		return nil, clienterrors.New(
			clienterrors.ErrorParsingDynamicArgs,
			"Error parsing BootcPreManifestJobResult from dynamic args: "+err.Error(),
			nil,
		)
	}

	if preManifestResult.JobError != nil {
		return nil, clienterrors.New(
			clienterrors.ErrorJobDependency,
			"BootcPreManifest dependency failed",
			preManifestResult.JobError.Reason,
		)
	}

	return preManifestResult.ContainerResolveJobArgs, nil
}
