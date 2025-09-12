package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/runner"
)

type BootcDiskImage struct {
	Base

	PartitionTable *disk.PartitionTable

	ContainerSource      *container.SourceSpec
	BuildContainerSource *container.SourceSpec

	// Customizations
	OSCustomizations manifest.OSCustomizations
}

func NewBootcDiskImage(platform platform.Platform, filename string, container container.SourceSpec, buildContainer container.SourceSpec) *BootcDiskImage {
	return &BootcDiskImage{
		Base:                 NewBase("bootc-raw-image", platform, filename),
		ContainerSource:      &container,
		BuildContainerSource: &buildContainer,
		OSCustomizations: manifest.OSCustomizations{
			MountConfiguration: osbuild.MOUNT_CONFIGURATION_UNITS, // default use mount units for bootc disk images
		},
	}
}

func (img *BootcDiskImage) InstantiateManifestFromContainers(m *manifest.Manifest,
	containers []container.SourceSpec,
	runner runner.Runner,
	rng *rand.Rand) error {

	policy := img.OSCustomizations.SELinux
	if img.OSCustomizations.BuildSELinux != "" {
		policy = img.OSCustomizations.BuildSELinux
	}

	var copyFilesFrom map[string][]string
	var ensureDirs []*fsnode.Directory

	var customSourcePipeline = ""
	if *img.ContainerSource != *img.BuildContainerSource {
		// If we're using a different build container from the target container then we copy
		// the bootc customization file directories from the target container. This includes the
		// bootc install customization, and /usr/lib/ostree/prepare-root.conf which configures
		// e.g. composefs and fs-verity setup.
		//
		// To ensure that these copies never fail we also create the source and target
		// directories as needed.

		pipelineName := "target"
		// files to copy have slash at end to copy directory contents, not directory itself
		copyFiles := []string{"/usr/lib/bootc/install/", "/usr/lib/ostree/"}
		ensureDirPaths := []string{"/usr/lib/bootc/install", "/usr/lib/ostree"}

		copyFilesFrom = map[string][]string{pipelineName: copyFiles}
		for _, path := range ensureDirPaths {
			// Note: Mode/User/Group must be nil here to make  GenDirectoryNodesStages use dirExistOk
			dir, err := fsnode.NewDirectory(path, nil, nil, nil, true)
			if err != nil {
				return err
			}
			ensureDirs = append(ensureDirs, dir)
		}

		targetContainers := []container.SourceSpec{*img.ContainerSource}
		targetBuildPipeline := manifest.NewBuildFromContainer(m, runner, targetContainers,
			&manifest.BuildOptions{
				PipelineName:       pipelineName,
				ContainerBuildable: true,
				SELinuxPolicy:      policy,
				EnsureDirs:         ensureDirs,
			})
		targetBuildPipeline.Checkpoint()

		customSourcePipeline = targetBuildPipeline.Name()
	}

	buildContainers := []container.SourceSpec{*img.BuildContainerSource}
	buildPipeline := manifest.NewBuildFromContainer(m, runner, buildContainers,
		&manifest.BuildOptions{
			ContainerBuildable: true,
			SELinuxPolicy:      policy,
			CopyFilesFrom:      copyFilesFrom,
			EnsureDirs:         ensureDirs,
		})

	buildPipeline.Checkpoint()

	// In the bootc flow, we reuse the host container context for tools;
	// this is signified by passing nil to the below pipelines.
	var hostPipeline manifest.Build

	rawImage := manifest.NewRawBootcImage(buildPipeline, containers, img.platform)
	if customSourcePipeline != "" {
		rawImage.SourcePipeline = customSourcePipeline
	}
	rawImage.PartitionTable = img.PartitionTable
	rawImage.Users = img.OSCustomizations.Users
	rawImage.Groups = img.OSCustomizations.Groups
	rawImage.Files = img.OSCustomizations.Files
	rawImage.Directories = img.OSCustomizations.Directories
	rawImage.KernelOptionsAppend = img.OSCustomizations.KernelOptionsAppend
	rawImage.SELinux = img.OSCustomizations.SELinux
	rawImage.MountConfiguration = img.OSCustomizations.MountConfiguration

	// In BIB, we export multiple images from the same pipeline so we use the
	// filename as the basename for each export and set the extensions based on
	// each file format.
	fileBasename := img.filename
	rawImage.SetFilename(fmt.Sprintf("%s.raw", fileBasename))

	qcow2Pipeline := manifest.NewQCOW2(hostPipeline, rawImage)
	qcow2Pipeline.Compat = img.platform.GetQCOW2Compat()
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
