package image

import (
	"math/rand"

	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type BaseContainer struct {
	Base
	OSCustomizations           manifest.OSCustomizations
	OCIContainerCustomizations manifest.OCIContainerCustomizations
	Environment                environment.Environment
}

func NewBaseContainer(platform platform.Platform, filename string) *BaseContainer {
	return &BaseContainer{
		Base: NewBase("base-container", platform, filename),
	}
}

func (img *BaseContainer) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, img.BuildOptions)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(buildPipeline, img.platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment

	ociPipeline := manifest.NewOCIContainer(buildPipeline, osPipeline)
	ociPipeline.OCIContainerCustomizations = img.OCIContainerCustomizations

	ociPipeline.SetFilename(img.filename)
	artifact := ociPipeline.Export()

	return artifact, nil
}
