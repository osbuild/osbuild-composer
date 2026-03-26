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

	// No-op: no specs to resolve
	if len(args.Specs) == 0 {
		return nil
	}

	logWithId.Infof("Resolving containers (%d)", len(args.Specs))

	result.Specs = make([]worker.ContainerSpec, len(args.Specs))

	resolver := container.NewResolver(args.Arch)
	resolver.AuthFilePath = impl.AuthFilePath

	for _, s := range args.Specs {
		resolver.Add(s.ToVendorSourceSpec())
	}

	specs, err := resolver.Finish()

	if err != nil {
		result.JobError = clienterrors.New(clienterrors.ErrorContainerResolution, err.Error(), nil)
		return fmt.Errorf("Error resolving containers: %v", err)
	}

	for i, spec := range specs {
		result.Specs[i] = worker.ContainerSpecFromVendorSpec(spec)
	}

	return nil
}
