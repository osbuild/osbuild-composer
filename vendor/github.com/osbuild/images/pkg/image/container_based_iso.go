package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/runner"
)

type ContainerBasedIso struct {
	Base

	// Container source for the OS tree
	ContainerSource container.SourceSpec

	Product string
	Version string
	Release string

	ISOLabel string

	RootfsCompression string
	RootfsType        manifest.ISORootfsType

	KernelPath    string
	InitramfsPath string
}

func NewContainerBasedIso(platform platform.Platform, filename string, container container.SourceSpec) *ContainerBasedIso {
	return &ContainerBasedIso{
		Base:            NewBase("container-based-iso", platform, filename),
		ContainerSource: container,
	}
}

func (img *ContainerBasedIso) InstantiateManifestFromContainer(m *manifest.Manifest,
	containers []container.SourceSpec,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	cnts := []container.SourceSpec{img.ContainerSource}

	// org.osbuild.grub2.iso.legacy fails to run with empty kernel options, so let's use something harmless
	kernelOpts := []string{
		fmt.Sprintf("root=live:CDLABEL=%s", img.ISOLabel),
		"rd.live.image",
		"quiet",
		"rhgb",
		"enforcing=0",
	}

	buildPipeline := manifest.NewBuildFromContainer(m, runner, cnts,
		&manifest.BuildOptions{
			ContainerBuildable: true,
		})

	osTreePipeline := manifest.NewOSFromContainer("os-tree", buildPipeline, &img.ContainerSource)

	product := img.Product
	if product == "" {
		product = "OS"
	}
	version := img.Version
	if version == "" {
		version = img.Release
	}
	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, product, version)
	bootTreePipeline.Platform = img.platform
	bootTreePipeline.UEFIVendor = img.platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.ISOLabel
	bootTreePipeline.KernelOpts = kernelOpts

	isoTreePipeline := manifest.NewISOTree(buildPipeline, osTreePipeline, bootTreePipeline)
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.Release
	isoTreePipeline.Product = img.Product
	isoTreePipeline.Version = img.Version
	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.RootfsType
	isoTreePipeline.KernelPath = img.KernelPath
	isoTreePipeline.InitramfsPath = img.InitramfsPath
	isoTreePipeline.KernelOpts = kernelOpts

	isoCustomizations := manifest.ISOCustomizations{
		Label:    img.ISOLabel,
		BootType: manifest.Grub2ISOBoot,
	}

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, isoCustomizations)
	isoPipeline.SetFilename(img.filename)
	artifact := isoPipeline.Export()

	return artifact, nil
}
