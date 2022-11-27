package main

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type ContainerResolveJobImpl struct {
	AuthFilePath string
}

func (impl *ContainerResolveJobImpl) Run(ctx context.Context, job *worker.Job) (interface{}, error) {
	logWithId := logrus.WithField("jobId", job.Id())
	var args worker.ContainerResolveJob
	err := job.Args(&args)
	if err != nil {
		return nil, err
	}

	result := worker.ContainerResolveJobResult{
		Specs: make([]worker.ContainerSpec, len(args.Specs)),
	}

	logWithId.Infof("Resolving containers (%d)", len(args.Specs))

	resolver := container.NewResolver(args.Arch)
	resolver.AuthFilePath = impl.AuthFilePath

	for _, s := range args.Specs {
		resolver.Add(s.Source, s.Name, s.TLSVerify)
	}

	specs, err := resolver.Finish()

	if err != nil {
		result.JobError = clienterrors.WorkerClientError(clienterrors.ErrorContainerResolution, err.Error(), nil)
	} else {
		for i, spec := range specs {
			result.Specs[i] = worker.ContainerSpec{
				Source:    spec.Source,
				Name:      spec.LocalName,
				TLSVerify: spec.TLSVerify,
				ImageID:   spec.ImageID,
				Digest:    spec.Digest,
			}
		}
	}

	return &result, nil
}
