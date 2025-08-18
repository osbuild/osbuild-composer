package image

import (
	"fmt"
	"math/rand"
	"path/filepath"

	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
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
	Platform                platform.Platform
	OSCustomizations        manifest.OSCustomizations
	InstallerCustomizations manifest.InstallerCustomizations
	Environment             environment.Environment
	Workload                workload.Workload

	ExtraBasePackages rpmmd.PackageSet

	Kickstart *kickstart.Options

	RootfsCompression string

	ISOLabel  string
	Product   string
	Variant   string
	OSVersion string
	Release   string
	Preview   bool

	Filename string
}

func NewAnacondaTarInstaller() *AnacondaTarInstaller {
	return &AnacondaTarInstaller{
		Base: NewBase("image-installer"),
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

	if img.Kickstart.Unattended {
		// if we're building an unattended installer, override the
		// ISORootKickstart option
		img.InstallerCustomizations.ISORootKickstart = true
	}

	if img.InstallerCustomizations.ISORootKickstart {
		// kickstart file will be in the iso root and not interactive-defaults,
		// so let's make sure the kickstart path option is set
		if img.Kickstart.Path == "" {
			img.Kickstart.Path = osbuild.KickstartPathOSBuild
		}
	}

	anacondaPipeline := manifest.NewAnacondaInstaller(
		manifest.AnacondaInstallerTypePayload,
		buildPipeline,
		img.Platform,
		repos,
		"kernel",
		img.Product,
		img.OSVersion,
		img.Preview,
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
	anacondaPipeline.Variant = img.Variant
	anacondaPipeline.Biosdevname = (img.Platform.GetArch() == arch.ARCH_X86_64)

	anacondaPipeline.InstallerCustomizations = img.InstallerCustomizations

	if img.OSCustomizations.FIPS {
		anacondaPipeline.InstallerCustomizations.EnabledAnacondaModules = append(
			anacondaPipeline.InstallerCustomizations.EnabledAnacondaModules,
			anaconda.ModuleSecurity,
		)
	}

	anacondaPipeline.Locale = img.OSCustomizations.Language

	tarPath := "/liveimg.tar.gz"

	if !img.InstallerCustomizations.ISORootKickstart {
		payloadPath := filepath.Join("/run/install/repo/", tarPath)
		anacondaPipeline.InteractiveDefaults = manifest.NewAnacondaInteractiveDefaults(fmt.Sprintf("file://%s", payloadPath))
	}

	anacondaPipeline.Checkpoint()

	var rootfsImagePipeline *manifest.ISORootfsImg
	switch img.InstallerCustomizations.ISORootfsType {
	case manifest.SquashfsExt4Rootfs:
		rootfsImagePipeline = manifest.NewISORootfsImg(buildPipeline, anacondaPipeline)
		rootfsImagePipeline.Size = 5 * datasizes.GibiByte
	default:
	}

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.Product, img.OSVersion)
	bootTreePipeline.Platform = img.Platform
	bootTreePipeline.UEFIVendor = img.Platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.ISOLabel
	bootTreePipeline.DefaultMenu = img.InstallerCustomizations.DefaultMenu

	kernelOpts := []string{fmt.Sprintf("inst.stage2=hd:LABEL=%s", img.ISOLabel)}
	if img.InstallerCustomizations.ISORootKickstart {
		kernelOpts = append(kernelOpts, fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", img.ISOLabel, img.Kickstart.Path))
	}
	if img.OSCustomizations.FIPS {
		kernelOpts = append(kernelOpts, "fips=1")
	}
	kernelOpts = append(kernelOpts, img.InstallerCustomizations.AdditionalKernelOpts...)
	kernelOpts = append(kernelOpts, img.OSCustomizations.KernelOptionsAppend...)
	bootTreePipeline.KernelOpts = kernelOpts

	osPipeline := manifest.NewOS(buildPipeline, img.Platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.Workload = img.Workload

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, anacondaPipeline, rootfsImagePipeline, bootTreePipeline)
	// TODO: the partition table is required - make it a ctor arg or set a default one in the pipeline
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.Release
	isoTreePipeline.Kickstart = img.Kickstart
	isoTreePipeline.PayloadPath = tarPath
	if img.InstallerCustomizations.ISORootKickstart {
		isoTreePipeline.Kickstart.Path = img.Kickstart.Path
	}

	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.InstallerCustomizations.ISORootfsType

	isoTreePipeline.OSPipeline = osPipeline
	isoTreePipeline.KernelOpts = img.InstallerCustomizations.AdditionalKernelOpts
	isoTreePipeline.KernelOpts = append(isoTreePipeline.KernelOpts, img.OSCustomizations.KernelOptionsAppend...)
	if img.OSCustomizations.FIPS {
		isoTreePipeline.KernelOpts = append(isoTreePipeline.KernelOpts, "fips=1")
	}

	isoTreePipeline.ISOBoot = img.InstallerCustomizations.ISOBoot

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.ISOLabel)
	isoPipeline.SetFilename(img.Filename)
	isoPipeline.ISOBoot = img.InstallerCustomizations.ISOBoot

	artifact := isoPipeline.Export()

	return artifact, nil
}
