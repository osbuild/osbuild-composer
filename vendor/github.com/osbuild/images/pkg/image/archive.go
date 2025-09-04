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

type Archive struct {
	Base
	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment
	Compression      string

	OSVersion string
}

func NewArchive(platform platform.Platform, filename string) *Archive {
	return &Archive{
		Base: NewBase("archive", platform, filename),
	}
}

func (img *Archive) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(buildPipeline, img.platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.OSVersion = img.OSVersion

	tarPipeline := manifest.NewTar(buildPipeline, osPipeline, "archive")

	compressionPipeline := GetCompressionPipeline(img.Compression, buildPipeline, tarPipeline)
	compressionPipeline.SetFilename(img.filename)

	return compressionPipeline.Export(), nil
}
