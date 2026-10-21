package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/runner"
)

type ContainerBasedIso struct {
	Base

	// Container source for the OS tree
	ContainerSource container.SourceSpec

	// PayloadContainer is an optional container to embed in the image's
	// container storage (for bootc installer scenarios where the payload
	// container needs to be available at install time).
	PayloadContainer *container.SourceSpec

	ISOCustomizations       manifest.ISOCustomizations
	InstallerCustomizations manifest.InstallerCustomizations

	RootfsCompression string

	KernelPath    string
	KernelOpts    []string
	InitramfsPath string

	// Custom ISO bootloader menus override the default menus
	Grub2MenuEntries []manifest.ISOGrub2MenuEntry
}

func NewContainerBasedIso(platform platform.Platform, filename string, container container.SourceSpec, buildOpts *manifest.BuildOptions) *ContainerBasedIso {
	return &ContainerBasedIso{
		Base:            NewBase("container-based-iso", platform, filename),
		ContainerSource: container,
	}
}

// Bootloaders returns the list of configured bootloaders for the platform
func (img *ContainerBasedIso) Bootloaders(buildPipeline manifest.Build, kernelOpts []string) []manifest.ISOBootloader {
	ibo := ISOBootloaders{
		InstallerCustomizations: &img.InstallerCustomizations,
		ISOCustomizations:       &img.ISOCustomizations,
		Custom:                  img.Grub2MenuEntries,
	}

	return ibo.Bootloaders(buildPipeline, img.platform, kernelOpts)
}

func (img *ContainerBasedIso) InstantiateManifestFromContainer(m *manifest.Manifest,
	containers []container.SourceSpec,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	cnts := []container.SourceSpec{img.ContainerSource}

	buildOptions := img.BuildOptions
	if buildOptions == nil {
		buildOptions = &manifest.BuildOptions{}
	}
	buildOptions.ContainerBuildable = true
	buildPipeline := manifest.NewBuildFromContainer(m, runner, cnts, buildOptions)
	osTreePipeline := manifest.NewOSFromContainer("os-tree", buildPipeline, &img.ContainerSource, img.platform)
	osTreePipeline.PayloadContainer = img.PayloadContainer

	kernelOpts := []string{
		fmt.Sprintf("root=live:CDLABEL=%s", img.ISOCustomizations.Label),
		"rd.live.image",
		"quiet",
		"rhgb",
		"enforcing=0",
	}
	kernelOpts = append(kernelOpts, img.KernelOpts...)
	kernelOpts = append(kernelOpts, img.InstallerCustomizations.KernelOptionsAppend...)

	// Setup the bootloaders
	bootloaders := img.Bootloaders(buildPipeline, kernelOpts)

	isoTreePipeline := manifest.NewISOTree(buildPipeline, osTreePipeline, bootloaders)
	isoTreePipeline.PartitionTable = disk.EFIBootPartitionTable(rng)
	isoTreePipeline.Release = img.InstallerCustomizations.Release
	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.ISOCustomizations.RootfsType
	isoTreePipeline.RootfsExcludes = img.ISOCustomizations.RootfsExcludes
	isoTreePipeline.ErofsOptions = &img.ISOCustomizations.ErofsOptions
	isoTreePipeline.KernelPath = img.KernelPath
	isoTreePipeline.InitramfsPath = img.InitramfsPath

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.ISOCustomizations)
	isoPipeline.SetFilename(img.filename)
	artifact := isoPipeline.Export()

	return artifact, nil
}
