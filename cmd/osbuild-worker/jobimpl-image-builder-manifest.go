package main

import (
	"os"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/sirupsen/logrus"
)

type ImageBuilderManifestJobImpl struct {
}

func (impl *ImageBuilderManifestJobImpl) Run(job worker.Job) error {
	logWithId := logrus.WithField("jobId", job.Id().String())

	result := &worker.ImageBuilderManifestJobResult{
		Manifest: nil,
		ManifestInfo: worker.ManifestInfo{
			OSBuildComposerVersion: common.BuildVersion(),
			// TODO: add image-builder version fields when we get
			// machine-readable version output from image-builder-cli
		},
	}

	defer func() {
		err := job.Finish(&result)
		if err != nil {
			logWithId.Errorf("Error reporting job result: %v", err)
		}
	}()

	var args worker.ImageBuilderManifestJob
	err := job.Args(&args)
	if err != nil {
		return err
	}

	manifest, err := worker.RunImageBuilderManifest(args.Args, args.ExtraEnv, os.Stderr)
	if err != nil {
		result.JobError = workerClientErrorFrom(err, logWithId)
	}
	result.Manifest = manifest

	return nil
}
