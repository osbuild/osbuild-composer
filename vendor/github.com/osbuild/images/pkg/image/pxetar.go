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

type PXETar struct {
	Base
	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment
	Compression      string

	OSVersion string
}

func NewPXETar(platform platform.Platform, filename string) *PXETar {
	return &PXETar{
		Base: NewBase("pxetar", platform, filename),
	}
}

func (img *PXETar) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, img.BuildOptions)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(buildPipeline, img.platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.OSVersion = img.OSVersion
	if osPipeline.OSCustomizations.KernelName == "" {
		// PXETree needs a kernel and initrd, fall back to default name if none set.
		osPipeline.OSCustomizations.KernelName = "kernel"
	}

	pxeTreePipeline := manifest.NewPXETree(buildPipeline, osPipeline)
	// TODO
	// - Setup compresstion (squashfs/erofs, etc.)

	// Add the PXETree dracut config to the os pipeline which is what generates the initrd
	osPipeline.OSCustomizations.DracutConf = append(osPipeline.OSCustomizations.DracutConf,
		pxeTreePipeline.DracutConfStageOptions())

	tarPipeline := manifest.NewTar(buildPipeline, pxeTreePipeline, "tar")

	compressionPipeline := GetCompressionPipeline(img.Compression, buildPipeline, tarPipeline)
	compressionPipeline.SetFilename(img.filename)

	return compressionPipeline.Export(), nil
}
