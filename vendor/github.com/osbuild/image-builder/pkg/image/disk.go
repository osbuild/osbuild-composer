package image

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"

	"github.com/osbuild/image-builder/internal/environment"
	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/runner"
)

type SysextConfig struct {
	Name                      string
	Format                    string
	ExtensionReleaseID        string
	ExtensionReleaseVersionID string
	Paths                     []string
	ExcludePaths              []string
	PackageSet                rpmmd.PackageSet
	Standalone                bool
}

type PartitionConfig struct {
	Name        string
	Mountpoint  string
	Filename    string
	Compression string
}

type FileConfig struct {
	Name        string
	Path        string
	Filename    string
	Compression string
}

type DiskImage struct {
	Base

	PartitionTable     *disk.PartitionTable
	OSCustomizations   manifest.OSCustomizations
	DiskCustomizations manifest.DiskCustomizations
	Environment        environment.Environment
	Compression        string

	Sysexts    []SysextConfig
	Partitions []PartitionConfig
	Files      []FileConfig

	// Control the VPC subformat use of force_size
	VPCForceSize *bool
	PartTool     osbuild.PartTool

	NoBLS     bool
	OSProduct string
	OSVersion string
	OSNick    string
}

func NewDiskImage(platform platform.Platform, filename string) *DiskImage {
	return &DiskImage{
		Base:     NewBase("disk", platform, filename),
		PartTool: osbuild.PTSfdisk,
	}
}

func (img *DiskImage) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {

	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, img.BuildOptions)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(buildPipeline, img.platform, repos)
	osPipeline.PartitionTable = img.PartitionTable
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.DiskCustomizations = img.DiskCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.OSProduct = img.OSProduct
	osPipeline.OSVersion = img.OSVersion
	osPipeline.OSNick = img.OSNick

	for _, sysext := range img.Sysexts {
		var depsolveRef manifest.Pipeline
		if !sysext.Standalone {
			depsolveRef = osPipeline
		}
		sp := manifest.NewSysextPipelines(buildPipeline, img.platform, repos, osPipeline, depsolveRef, sysext.Name)
		sp.Tree.Customizations.PackageSet = sysext.PackageSet
		sp.Tree.Customizations.BaseRPMOptions = img.OSCustomizations.BaseRPMOptions.Clone()
		sp.Prep.Customizations.Paths = sysext.Paths
		sp.Prep.Customizations.ExcludePaths = sysext.ExcludePaths
		sp.Prep.Customizations.ExtensionRelease.Vars.ID = sysext.ExtensionReleaseID
		sp.Prep.Customizations.ExtensionRelease.Vars.VersionID = sysext.ExtensionReleaseVersionID
		sp.Prep.Customizations.SELinux = img.OSCustomizations.SELinux

		switch sysext.Format {
		case "erofs":
			erofsPipeline := manifest.NewErofs(buildPipeline, sp.Prep, SysextPipelineName(sysext.Name, sysext.Format))
			erofsPipeline.SetFilename("sysext-" + sysext.Name + ".erofs")
			erofsPipeline.Export()
		default:
			return nil, fmt.Errorf("unsupported sysext format %q for %q", sysext.Format, sysext.Name)
		}
	}

	rawImagePipeline := manifest.NewRawImage(buildPipeline, osPipeline, img.DiskCustomizations)

	for _, sp := range img.Partitions {
		partPipelineName := PartitionPipelineName(sp.Name, "")
		partPipeline := manifest.NewPartitionImage(buildPipeline, rawImagePipeline, sp.Mountpoint, img.PartitionTable, partPipelineName)
		partPipeline.SetFilename(sp.Name + ".raw")
		var exportPipeline manifest.FilePipeline = partPipeline
		if sp.Compression != "" {
			exportPipeline = GetCompressionPipeline(sp.Compression, buildPipeline, partPipeline, PartitionPipelineName(sp.Name, sp.Compression))
			exportPipeline.SetFilename(fmt.Sprintf("%s.raw.%s", sp.Name, compressionExt(sp.Compression)))
		}
		if sp.Filename != "" {
			exportPipeline.SetFilename(sp.Filename)
		}
		exportPipeline.Export()
	}

	for _, fc := range img.Files {
		prepPipeline := manifest.NewFilePrep(buildPipeline, rawImagePipeline, fc.Path, img.PartitionTable, FilePrepPipelineName(fc.Name))
		var exportPipeline manifest.FilePipeline
		if fc.Compression != "" {
			exportPipeline = GetCompressionPipeline(fc.Compression, buildPipeline, prepPipeline, FilePipelineName(fc.Name, fc.Compression))
			exportPipeline.SetFilename(fmt.Sprintf("%s.%s", filepath.Base(fc.Path), compressionExt(fc.Compression)))
		} else {
			exportPipeline = manifest.NewCopyFile(buildPipeline, prepPipeline, FilePipelineName(fc.Name, ""))
		}
		if fc.Filename != "" {
			exportPipeline.SetFilename(fc.Filename)
		}
		exportPipeline.Export()
	}

	var imagePipeline manifest.FilePipeline
	switch img.platform.GetImageFormat() {
	case platform.FORMAT_RAW:
		imagePipeline = rawImagePipeline
	case platform.FORMAT_QCOW2:
		qcow2Pipeline := manifest.NewQCOW2(buildPipeline, rawImagePipeline)
		qcow2Pipeline.Compat = img.platform.GetQCOW2Compat()
		imagePipeline = qcow2Pipeline
	case platform.FORMAT_VAGRANT_LIBVIRT:
		qcow2Pipeline := manifest.NewQCOW2(buildPipeline, rawImagePipeline)
		qcow2Pipeline.Compat = img.platform.GetQCOW2Compat()

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
		tarPipeline.SetFilename(img.filename)

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
		ovfPipeline.VMWareOSType = img.DiskCustomizations.OVFVMWare.OSType
		ovfPipeline.VMWareVirtualHardwareVersion = img.DiskCustomizations.OVFVMWare.VirtualHardwareVersion
		tarPipeline := manifest.NewTar(buildPipeline, ovfPipeline, "archive")
		tarPipeline.Format = osbuild.TarArchiveFormatUstar
		tarPipeline.SetFilename(img.filename)
		extLess := strings.TrimSuffix(img.filename, filepath.Ext(img.filename))
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

	compressionPipeline := GetCompressionPipeline(img.Compression, buildPipeline, imagePipeline, "")
	compressionPipeline.SetFilename(img.filename)

	return compressionPipeline.Export(), nil
}
