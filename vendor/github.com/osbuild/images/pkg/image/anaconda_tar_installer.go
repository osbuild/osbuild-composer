package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

func efiBootPartitionTable(rng *rand.Rand) *disk.PartitionTable {
	var efibootImageSize uint64 = 20 * datasizes.MebiByte
	return &disk.PartitionTable{
		Size: efibootImageSize,
		Partitions: []disk.Partition{
			{
				Start: 0,
				Size:  efibootImageSize,
				Payload: &disk.Filesystem{
					Type:       "vfat",
					Mountpoint: "/",
					UUID:       disk.NewVolIDFromRand(rng),
				},
			},
		},
	}
}

type AnacondaTarInstaller struct {
	Base
	OSCustomizations        manifest.OSCustomizations
	InstallerCustomizations manifest.InstallerCustomizations
	Environment             environment.Environment

	ExtraBasePackages rpmmd.PackageSet

	Kickstart *kickstart.Options

	RootfsCompression string
}

func NewAnacondaTarInstaller(platform platform.Platform, filename string) *AnacondaTarInstaller {
	return &AnacondaTarInstaller{
		Base: NewBase("image-installer", platform, filename),
	}
}

func (img *AnacondaTarInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	if img.Kickstart == nil {
		img.Kickstart = &kickstart.Options{}
	}

	if img.Kickstart.Path == "" {
		img.Kickstart.Path = osbuild.KickstartPathOSBuild
	}

	anacondaPipeline := manifest.NewAnacondaInstaller(
		manifest.AnacondaInstallerTypePayload,
		buildPipeline,
		img.platform,
		repos,
		"kernel",
		img.InstallerCustomizations,
	)

	anacondaPipeline.ExtraPackages = img.ExtraBasePackages.Include
	anacondaPipeline.ExcludePackages = img.ExtraBasePackages.Exclude
	anacondaPipeline.ExtraRepos = img.ExtraBasePackages.Repositories
	if img.Kickstart != nil {
		anacondaPipeline.InteractiveDefaultsKickstart = &kickstart.Options{
			Users:  img.Kickstart.Users,
			Groups: img.Kickstart.Groups,
		}
	}
	anacondaPipeline.Biosdevname = (img.platform.GetArch() == arch.ARCH_X86_64)

	if img.OSCustomizations.FIPS {
		anacondaPipeline.InstallerCustomizations.EnabledAnacondaModules = append(
			anacondaPipeline.InstallerCustomizations.EnabledAnacondaModules,
			anaconda.ModuleSecurity,
		)
	}

	anacondaPipeline.Locale = img.OSCustomizations.Language

	tarPath := "/liveimg.tar.gz"

	anacondaPipeline.Checkpoint()

	var rootfsImagePipeline *manifest.ISORootfsImg
	switch img.InstallerCustomizations.ISORootfsType {
	case manifest.SquashfsExt4Rootfs:
		rootfsImagePipeline = manifest.NewISORootfsImg(buildPipeline, anacondaPipeline)
		rootfsImagePipeline.Size = 5 * datasizes.GibiByte
	default:
	}

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
	bootTreePipeline.Platform = img.platform
	bootTreePipeline.UEFIVendor = img.platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.InstallerCustomizations.ISOLabel
	bootTreePipeline.DefaultMenu = img.InstallerCustomizations.DefaultMenu

	kernelOpts := []string{
		fmt.Sprintf("inst.stage2=hd:LABEL=%s", img.InstallerCustomizations.ISOLabel),
		fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", img.InstallerCustomizations.ISOLabel, img.Kickstart.Path),
	}

	if img.OSCustomizations.FIPS {
		kernelOpts = append(kernelOpts, "fips=1")
	}
	kernelOpts = append(kernelOpts, img.InstallerCustomizations.AdditionalKernelOpts...)
	kernelOpts = append(kernelOpts, img.OSCustomizations.KernelOptionsAppend...)
	bootTreePipeline.KernelOpts = kernelOpts

	osPipeline := manifest.NewOS(buildPipeline, img.platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, anacondaPipeline, rootfsImagePipeline, bootTreePipeline)
	// TODO: the partition table is required - make it a ctor arg or set a default one in the pipeline
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.InstallerCustomizations.Release
	isoTreePipeline.Kickstart = img.Kickstart
	isoTreePipeline.PayloadPath = tarPath
	isoTreePipeline.Kickstart.Path = img.Kickstart.Path

	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.InstallerCustomizations.ISORootfsType

	isoTreePipeline.OSPipeline = osPipeline
	isoTreePipeline.KernelOpts = img.InstallerCustomizations.AdditionalKernelOpts
	isoTreePipeline.KernelOpts = append(isoTreePipeline.KernelOpts, img.OSCustomizations.KernelOptionsAppend...)
	if img.OSCustomizations.FIPS {
		isoTreePipeline.KernelOpts = append(isoTreePipeline.KernelOpts, "fips=1")
	}

	isoTreePipeline.ISOBoot = img.InstallerCustomizations.ISOBoot

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.InstallerCustomizations.ISOLabel)
	isoPipeline.SetFilename(img.filename)
	isoPipeline.ISOBoot = img.InstallerCustomizations.ISOBoot

	artifact := isoPipeline.Export()

	return artifact, nil
}
