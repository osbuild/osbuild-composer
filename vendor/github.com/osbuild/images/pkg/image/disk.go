package image

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type DiskImage struct {
	Base
	Platform              platform.Platform
	PartitionTable        *disk.PartitionTable
	OSCustomizations      manifest.OSCustomizations
	Environment           environment.Environment
	ImgTypeCustomizations manifest.OSCustomizations
	Filename              string
	Compression           string

	// Control the VPC subformat use of force_size
	VPCForceSize *bool
	PartTool     osbuild.PartTool

	NoBLS     bool
	OSProduct string
	OSVersion string
	OSNick    string
}

func NewDiskImage() *DiskImage {
	return &DiskImage{
		Base:     NewBase("disk"),
		PartTool: osbuild.PTSfdisk,
	}
}

func (img *DiskImage) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {

	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(buildPipeline, img.Platform, repos)
	osPipeline.PartitionTable = img.PartitionTable
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.ImgTypeCustomizations = img.ImgTypeCustomizations
	osPipeline.OSProduct = img.OSProduct
	osPipeline.OSVersion = img.OSVersion
	osPipeline.OSNick = img.OSNick

	rawImagePipeline := manifest.NewRawImage(buildPipeline, osPipeline)
	rawImagePipeline.PartTool = img.PartTool

	var imagePipeline manifest.FilePipeline
	switch img.Platform.GetImageFormat() {
	case platform.FORMAT_RAW:
		imagePipeline = rawImagePipeline
	case platform.FORMAT_QCOW2:
		qcow2Pipeline := manifest.NewQCOW2(buildPipeline, rawImagePipeline)
		qcow2Pipeline.Compat = img.Platform.GetQCOW2Compat()
		imagePipeline = qcow2Pipeline
	case platform.FORMAT_VAGRANT_LIBVIRT:
		qcow2Pipeline := manifest.NewQCOW2(buildPipeline, rawImagePipeline)
		qcow2Pipeline.Compat = img.Platform.GetQCOW2Compat()

		vagrantPipeline := manifest.NewVagrant(buildPipeline, qcow2Pipeline, osbuild.VagrantProviderLibvirt, rng)

		tarPipeline := manifest.NewTar(buildPipeline, vagrantPipeline, "archive")
		tarPipeline.Format = osbuild.TarArchiveFormatUstar

		imagePipeline = tarPipeline
	case platform.FORMAT_VAGRANT_VIRTUALBOX:
		vmdkPipeline := manifest.NewVMDK(buildPipeline, rawImagePipeline)
		vmdkPipeline.SetFilename("box.vmdk")

		vagrantPipeline := manifest.NewVagrant(buildPipeline, vmdkPipeline, osbuild.VagrantProviderVirtualBox, rng)

		tarPipeline := manifest.NewTar(buildPipeline, vagrantPipeline, "archive")
		tarPipeline.Format = osbuild.TarArchiveFormatUstar
		tarPipeline.SetFilename(img.Filename)

		imagePipeline = tarPipeline
	case platform.FORMAT_VHD:
		vpcPipeline := manifest.NewVPC(buildPipeline, rawImagePipeline)
		vpcPipeline.ForceSize = img.VPCForceSize
		imagePipeline = vpcPipeline
	case platform.FORMAT_VMDK:
		imagePipeline = manifest.NewVMDK(buildPipeline, rawImagePipeline)
	case platform.FORMAT_OVA:
		vmdkPipeline := manifest.NewVMDK(buildPipeline, rawImagePipeline)
		ovfPipeline := manifest.NewOVF(buildPipeline, vmdkPipeline)
		tarPipeline := manifest.NewTar(buildPipeline, ovfPipeline, "archive")
		tarPipeline.Format = osbuild.TarArchiveFormatUstar
		tarPipeline.SetFilename(img.Filename)
		extLess := strings.TrimSuffix(img.Filename, filepath.Ext(img.Filename))
		// The .ovf descriptor needs to be the first file in the archive
		tarPipeline.Paths = []string{
			fmt.Sprintf("%s.ovf", extLess),
			fmt.Sprintf("%s.mf", extLess),
			fmt.Sprintf("%s.vmdk", extLess),
		}
		imagePipeline = tarPipeline
	case platform.FORMAT_GCE:
		// NOTE(akoutsou): temporary workaround; filename required for GCP
		// TODO: define internal raw filename on image type
		rawImagePipeline.SetFilename("disk.raw")
		tarPipeline := newGCETarPipelineForImg(buildPipeline, rawImagePipeline, "archive")
		imagePipeline = tarPipeline
	default:
		panic("invalid image format for image kind")
	}

	compressionPipeline := GetCompressionPipeline(img.Compression, buildPipeline, imagePipeline)
	compressionPipeline.SetFilename(img.Filename)

	return compressionPipeline.Export(), nil
}
