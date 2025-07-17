package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/runner"
)

// Legacy pipeline to create bootc images. This can be removed once
//
//	https://github.com/containers/bootc/pull/462
//
// or
//
//	https://www.mail-archive.com/qemu-devel@nongnu.org/msg1034508.html
//
// is available everyhwere
type BootcLegacyDiskImage struct {
	bootcImg *BootcDiskImage
}

func NewBootcLegacyDiskImage(real *BootcDiskImage) *BootcLegacyDiskImage {
	return &BootcLegacyDiskImage{
		bootcImg: real,
	}
}

func (img *BootcLegacyDiskImage) InstantiateManifestFromContainers(m *manifest.Manifest,
	containers []container.SourceSpec,
	runner runner.Runner,
	rng *rand.Rand) error {

	// XXX: hardcoded for now
	ref := "ostree/1/1/0"
	ostreeImg := &OSTreeDiskImage{
		Base:            NewBase("bootc-raw-image"),
		ContainerSource: img.bootcImg.ContainerSource,
		Ref:             ref,
		OSName:          "default",
	}
	ostreeImg.Platform = img.bootcImg.Platform
	ostreeImg.PartitionTable = img.bootcImg.PartitionTable
	ostreeImg.OSTreeDeploymentCustomizations.KernelOptionsAppend = img.bootcImg.OSCustomizations.KernelOptionsAppend
	ostreeImg.OSTreeDeploymentCustomizations.Users = img.bootcImg.OSCustomizations.Users
	ostreeImg.OSTreeDeploymentCustomizations.Groups = img.bootcImg.OSCustomizations.Groups

	buildPipeline := manifest.NewBuildFromContainer(m, runner, containers, &manifest.BuildOptions{ContainerBuildable: true})
	buildPipeline.Checkpoint()

	// In the bootc flow, we reuse the host container context for tools;
	// this is signified by passing nil to the below pipelines.
	var hostPipeline manifest.Build

	opts := &baseRawOstreeImageOpts{useBootupd: true}

	fileBasename := img.bootcImg.Filename

	// In BIB, we export multiple images from the same pipeline so we use the
	// filename as the basename for each export and set the extensions based on
	// each file format.
	baseImage := baseRawOstreeImage(ostreeImg, buildPipeline, opts)
	baseImage.SetFilename(fmt.Sprintf("%s.raw", fileBasename))

	qcow2Pipeline := manifest.NewQCOW2(hostPipeline, baseImage)
	qcow2Pipeline.Compat = ostreeImg.Platform.GetQCOW2Compat()
	qcow2Pipeline.SetFilename(fmt.Sprintf("%s.qcow2", fileBasename))

	vmdkPipeline := manifest.NewVMDK(hostPipeline, baseImage)
	vmdkPipeline.SetFilename(fmt.Sprintf("%s.vmdk", fileBasename))

	ovfPipeline := manifest.NewOVF(hostPipeline, vmdkPipeline)
	tarPipeline := manifest.NewTar(hostPipeline, ovfPipeline, "archive")
	tarPipeline.Format = osbuild.TarArchiveFormatUstar
	tarPipeline.SetFilename(fmt.Sprintf("%s.tar", fileBasename))
	// The .ovf descriptor needs to be the first file in the archive
	tarPipeline.Paths = []string{
		fmt.Sprintf("%s.ovf", fileBasename),
		fmt.Sprintf("%s.mf", fileBasename),
		fmt.Sprintf("%s.vmdk", fileBasename),
	}
	return nil
}
