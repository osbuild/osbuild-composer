package image

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/environment"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

type LiveImage struct {
	Base
	Platform         platform.Platform
	PartitionTable   *disk.PartitionTable
	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment
	Workload         workload.Workload
	Filename         string
}

func NewLiveImage() *LiveImage {
	return &LiveImage{
		Base: NewBase("live-image"),
	}
}

func (img *LiveImage) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(m, buildPipeline, img.Platform, repos)
	osPipeline.PartitionTable = img.PartitionTable
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.Workload = img.Workload

	imagePipeline := manifest.NewRawImage(m, buildPipeline, osPipeline)

	var artifact *artifact.Artifact
	switch img.Platform.GetImageFormat() {
	case platform.FORMAT_RAW:
		imagePipeline.Filename = img.Filename
		artifact = imagePipeline.Export()
	case platform.FORMAT_QCOW2:
		qcow2Pipeline := manifest.NewQCOW2(m, buildPipeline, imagePipeline)
		qcow2Pipeline.Filename = img.Filename
		qcow2Pipeline.Compat = img.Platform.GetQCOW2Compat()
		artifact = qcow2Pipeline.Export()
	case platform.FORMAT_VHD:
		vpcPipeline := manifest.NewVPC(m, buildPipeline, imagePipeline)
		vpcPipeline.Filename = img.Filename
		artifact = vpcPipeline.Export()
	case platform.FORMAT_VMDK:
		vmdkPipeline := manifest.NewVMDK(m, buildPipeline, imagePipeline)
		vmdkPipeline.Filename = img.Filename
		artifact = vmdkPipeline.Export()
	default:
		panic("invalid image format for image kind")
	}

	return artifact, nil
}
