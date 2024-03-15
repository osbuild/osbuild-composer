package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/runner"
)

type BootcDiskImage struct {
	*OSTreeDiskImage
}

func NewBootcDiskImage(container container.SourceSpec) *BootcDiskImage {
	// XXX: hardcoded for now
	ref := "ostree/1/1/0"

	return &BootcDiskImage{
		&OSTreeDiskImage{
			Base:            NewBase("bootc-raw-image"),
			ContainerSource: &container,
			Ref:             ref,
			OSName:          "default",
		},
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

	opts := &baseRawOstreeImageOpts{useBootupd: true}

	fileBasename := img.Filename

	// In BIB, we export multiple images from the same pipeline so we use the
	// filename as the basename for each export and set the extensions based on
	// each file format.
	baseImage := baseRawOstreeImage(img.OSTreeDiskImage, buildPipeline, opts)
	baseImage.SetFilename(fmt.Sprintf("%s.raw", fileBasename))

	qcow2Pipeline := manifest.NewQCOW2(hostPipeline, baseImage)
	qcow2Pipeline.Compat = img.Platform.GetQCOW2Compat()
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
