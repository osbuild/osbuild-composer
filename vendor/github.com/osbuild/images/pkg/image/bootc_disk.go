package image

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
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
	rng *rand.Rand) (*artifact.Artifact, error) {

	buildPipeline := manifest.NewBuildFromContainer(m, runner, containers, &manifest.BuildOptions{ContainerBuildable: true})
	buildPipeline.Checkpoint()

	// don't support compressing non-raw images
	imgFormat := img.Platform.GetImageFormat()
	if imgFormat == platform.FORMAT_UNSET {
		// treat unset as raw for this check
		imgFormat = platform.FORMAT_RAW
	}
	if imgFormat != platform.FORMAT_RAW && img.Compression != "" {
		panic(fmt.Sprintf("no compression is allowed with %q format for %q", imgFormat, img.name))
	}

	// In the bootc flow, we reuse the host container context for tools;
	// this is signified by passing nil to the below pipelines.
	var hostPipeline manifest.Build

	opts := &baseRawOstreeImageOpts{useBootupd: true}
	baseImage := baseRawOstreeImage(img.OSTreeDiskImage, buildPipeline, opts)

	// In BIB, we intend to export multiple images from the same pipeline and
	// this is the expected filename for the raw images. Set it so that it's
	// always disk.raw even when we're building a qcow2 or other image type.
	baseImage.SetFilename("disk.raw")
	switch imgFormat {
	case platform.FORMAT_QCOW2:
		qcow2Pipeline := manifest.NewQCOW2(hostPipeline, baseImage)
		qcow2Pipeline.Compat = img.Platform.GetQCOW2Compat()
		qcow2Pipeline.SetFilename(img.Filename)
		return qcow2Pipeline.Export(), nil
	// TODO: refactor to share this with disk.go; note here the build pipeline runs
	// on the host (that's the nil)
	case platform.FORMAT_VMDK:
		vmdkPipeline := manifest.NewVMDK(hostPipeline, baseImage)
		vmdkPipeline.SetFilename(img.Filename)
		return vmdkPipeline.Export(), nil
	case platform.FORMAT_OVA:
		vmdkPipeline := manifest.NewVMDK(hostPipeline, baseImage)
		ovfPipeline := manifest.NewOVF(hostPipeline, vmdkPipeline)
		tarPipeline := manifest.NewTar(hostPipeline, ovfPipeline, "archive")
		tarPipeline.Format = osbuild.TarArchiveFormatUstar
		tarPipeline.SetFilename(img.Filename)
		extLess := strings.TrimSuffix(img.Filename, filepath.Ext(img.Filename))
		// The .ovf descriptor needs to be the first file in the archive
		tarPipeline.Paths = []string{
			fmt.Sprintf("%s.ovf", extLess),
			fmt.Sprintf("%s.mf", extLess),
			fmt.Sprintf("%s.vmdk", extLess),
		}
		return tarPipeline.Export(), nil
	}

	switch img.Compression {
	case "xz":
		compressedImage := manifest.NewXZ(buildPipeline, baseImage)
		compressedImage.SetFilename(img.Filename)
		return compressedImage.Export(), nil
	case "":
		baseImage.SetFilename(img.Filename)
		return baseImage.Export(), nil
	default:
		panic(fmt.Sprintf("unsupported compression type %q on %q", img.Compression, img.name))
	}
}
