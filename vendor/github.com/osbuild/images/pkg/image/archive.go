package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type Archive struct {
	Base
	Platform         platform.Platform
	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment
	Workload         workload.Workload
	Filename         string
	Compression      string

	OSVersion string
}

func NewArchive() *Archive {
	return &Archive{
		Base: NewBase("archive"),
	}
}

func (img *Archive) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(buildPipeline, img.Platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.Workload = img.Workload
	osPipeline.OSVersion = img.OSVersion

	tarPipeline := manifest.NewTar(buildPipeline, osPipeline, "archive")
	tarPipeline.SetFilename(img.Filename)

	switch img.Compression {
	case "xz":
		tarPipeline.Compression = osbuild.TarArchiveCompressionXz
	case "gzip":
		tarPipeline.Compression = osbuild.TarArchiveCompressionGzip
	case "zstd":
		tarPipeline.Compression = osbuild.TarArchiveCompressionZstd
	case "":
		// this defaults to automatic compression based on filename which
		// has already been set
	default:
		// panic on unknown strings
		panic(fmt.Sprintf("unsupported compression type %q", img.Compression))
	}

	artifact := tarPipeline.Export()

	return artifact, nil
}
