package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
	"github.com/sirupsen/logrus"
)

type ImageBuilderManifestJobImpl struct {
	RepositoryMTLSConfig *RepositoryMTLSConfig
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

	if impl.RepositoryMTLSConfig != nil {
		for repoi, repo := range args.Args.Repositories {
			for _, baseurlstr := range repo.BaseURLs {
				match, err := impl.RepositoryMTLSConfig.CompareBaseURL(baseurlstr)
				if err != nil {
					result.JobError = clienterrors.New(clienterrors.ErrorInvalidRepositoryURL, "Repository URL is malformed", err.Error())
					return err
				}
				if match {
					impl.RepositoryMTLSConfig.SetupRepoSSL(&args.Args.Repositories[repoi])
				}
			}
		}
	}

	manifest, err := worker.RunImageBuilderManifest(args.Args, args.ExtraEnv, os.Stderr)
	if err != nil {
		result.JobError = workerClientErrorFrom(err, logWithId)
	}
	result.Manifest = manifest

	pipelineNames, err := parseManifestPipelines(manifest)
	if err != nil {
		// Not a critical failure. Log the error and continue. This will cause
		// the job log to be unordered.
		// For Koji builds, it will also fail to create package listings.
		logWithId.Warningf("failed to parse manifest for pipeline names: %v", err)
	}
	result.ManifestInfo.PipelineNames = pipelineNames

	return nil
}

// Parse the raw manifest into an incomplete representation of the manifest
// structure in order to retrieve the pipeline names in order. Any pipeline
// names that appear under the 'build' property of another pipeline are added
// to the build pipelines list. The rest are added to the payload pipelines.
func parseManifestPipelines(rawManifest []byte) (*worker.PipelineNames, error) {
	if len(rawManifest) == 0 {
		return nil, fmt.Errorf("manifest is empty")
	}

	// Partial manifest structure for extracting pipeline names.
	type partialManifest struct {
		Version   string `json:"version"`
		Pipelines []struct {
			Name  string `json:"name"`
			Build string `json:"build"`
		} `json:"pipelines"`
	}

	var pm partialManifest
	if err := json.Unmarshal(rawManifest, &pm); err != nil {
		return nil, err
	}

	if pm.Version != "2" {
		return nil, fmt.Errorf("unexpected manifest version: %s != 2", pm.Version)
	}

	if len(pm.Pipelines) == 0 {
		return nil, fmt.Errorf("no pipelines found")
	}

	// Collect all pipeline names in order and save build pipeline names in a
	// map so we can split them later.
	var allPipelineNames []string
	buildPipelineNames := make(map[string]bool)
	for _, pl := range pm.Pipelines {
		allPipelineNames = append(allPipelineNames, pl.Name)
		if pl.Build != "" {
			// The build property is in the form "name:build", where "build" is
			// the name of the build pipeline.
			parts := strings.SplitN(pl.Build, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("unexpected pipeline build property format: %s", pl.Build)
			}
			buildPipelineNames[parts[1]] = true
		}
	}

	var pipelineNames worker.PipelineNames
	for _, plName := range allPipelineNames {
		if buildPipelineNames[plName] {
			pipelineNames.Build = append(pipelineNames.Build, plName)
			continue
		}

		pipelineNames.Payload = append(pipelineNames.Payload, plName)
	}

	return &pipelineNames, nil
}
