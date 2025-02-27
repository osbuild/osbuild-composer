package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/runner"
)

type BootcDiskImage struct {
	Base

	Platform       platform.Platform
	PartitionTable *disk.PartitionTable

	Filename string

	ContainerSource *container.SourceSpec

	// Customizations
	KernelOptionsAppend []string

	// The users to put into the image, note that /etc/paswd (and friends)
	// will become unmanaged state by bootc when used
	Users  []users.User
	Groups []users.Group

	// Custom directories and files to create in the image
	Directories []*fsnode.Directory
	Files       []*fsnode.File

	// SELinux policy, when set it enables the labeling of the tree with the
	// selected profile
	SELinux string
}

func NewBootcDiskImage(container container.SourceSpec) *BootcDiskImage {
	return &BootcDiskImage{
		Base:            NewBase("bootc-raw-image"),
		ContainerSource: &container,
	}
}

func (img *BootcDiskImage) InstantiateManifestFromContainers(m *manifest.Manifest,
	containers []container.SourceSpec,
	runner runner.Runner,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildFromContainer(m, runner, containers, &manifest.BuildOptions{ContainerBuildable: true})
	buildPipeline.Checkpoint()

	// In the bootc flow, we reuse the host container context for tools;
	// this is signified by passing nil to the below pipelines.
	var hostPipeline manifest.Build

	rawImage := manifest.NewRawBootcImage(buildPipeline, containers, img.Platform)
	rawImage.PartitionTable = img.PartitionTable
	rawImage.Users = img.Users
	rawImage.Groups = img.Groups
	rawImage.Files = img.Files
	rawImage.Directories = img.Directories
	rawImage.KernelOptionsAppend = img.KernelOptionsAppend
	rawImage.SELinux = img.SELinux

	// In BIB, we export multiple images from the same pipeline so we use the
	// filename as the basename for each export and set the extensions based on
	// each file format.
	fileBasename := img.Filename
	rawImage.SetFilename(fmt.Sprintf("%s.raw", fileBasename))

	qcow2Pipeline := manifest.NewQCOW2(hostPipeline, rawImage)
	qcow2Pipeline.Compat = img.Platform.GetQCOW2Compat()
	qcow2Pipeline.SetFilename(fmt.Sprintf("%s.qcow2", fileBasename))

	vmdkPipeline := manifest.NewVMDK(hostPipeline, rawImage)
	vmdkPipeline.SetFilename(fmt.Sprintf("%s.vmdk", fileBasename))

	vhdPipeline := manifest.NewVPC(hostPipeline, rawImage)
	vhdPipeline.SetFilename(fmt.Sprintf("%s.vhd", fileBasename))

	ovfPipeline := manifest.NewOVF(hostPipeline, vmdkPipeline)
	tarPipeline := manifest.NewTar(hostPipeline, ovfPipeline, "archive")
	tarPipeline.Format = osbuild.TarArchiveFormatUstar
	tarPipeline.SetFilename(fmt.Sprintf("%s.tar", fileBasename))
	// The .ovf descriptor needs to be the first file in the archive
	tarPipeline.Paths = []string{
		fmt.Sprintf("%s.ovf", fileBasename),
		fmt.Sprintf("%s.mf", fileBasename),
		fmt.Sprintf("%s.vmdk", fileBasename),
		fmt.Sprintf("%s.vhd", fileBasename),
	}

	gcePipeline := newGCETarPipelineForImg(buildPipeline, rawImage, "gce")
	gcePipeline.SetFilename("image.tar.gz")

	return nil
}
