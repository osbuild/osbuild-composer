package image

import (
	"fmt"
	"math/rand"
	"path/filepath"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

func efiBootPartitionTable(rng *rand.Rand) *disk.PartitionTable {
	var efibootImageSize uint64 = 20 * common.MebiByte
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
	Platform         platform.Platform
	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment
	Workload         workload.Workload

	ExtraBasePackages rpmmd.PackageSet

	// If set, the kickstart file will be added to the bootiso-tree at the
	// default path for osbuild, otherwise any kickstart options will be
	// configured in the default location for interactive defaults in the
	// rootfs. Enabling UnattendedKickstart automatically enables this option
	// because automatic installations cannot be configured using interactive
	// defaults.
	ISORootKickstart bool
	Kickstart        *kickstart.Options

	SquashfsCompression string

	ISOLabel  string
	Product   string
	Variant   string
	OSVersion string
	Release   string
	Preview   bool

	Filename string

	AdditionalKernelOpts      []string
	AdditionalAnacondaModules []string
	DisabledAnacondaModules   []string
	AdditionalDracutModules   []string
	AdditionalDrivers         []string
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
	buildPipeline := manifest.NewBuild(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	if img.Kickstart == nil {
		img.Kickstart = &kickstart.Options{}
	}

	if img.Kickstart.Unattended {
		// if we're building an unattended installer, override the
		// ISORootKickstart option
		img.ISORootKickstart = true
	}

	if img.ISORootKickstart {
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
	anacondaPipeline.AdditionalAnacondaModules = img.AdditionalAnacondaModules
	if img.OSCustomizations.FIPS {
		anacondaPipeline.AdditionalAnacondaModules = append(
			anacondaPipeline.AdditionalAnacondaModules,
			anaconda.ModuleSecurity,
		)
	}
	anacondaPipeline.DisabledAnacondaModules = img.DisabledAnacondaModules
	anacondaPipeline.AdditionalDracutModules = img.AdditionalDracutModules
	anacondaPipeline.AdditionalDrivers = img.AdditionalDrivers

	tarPath := "/liveimg.tar.gz"

	if !img.ISORootKickstart {
		payloadPath := filepath.Join("/run/install/repo/", tarPath)
		anacondaPipeline.InteractiveDefaults = manifest.NewAnacondaInteractiveDefaults(fmt.Sprintf("file://%s", payloadPath))
	}

	anacondaPipeline.Checkpoint()

	rootfsImagePipeline := manifest.NewISORootfsImg(buildPipeline, anacondaPipeline)
	rootfsImagePipeline.Size = 5 * common.GibiByte

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.Product, img.OSVersion)
	bootTreePipeline.Platform = img.Platform
	bootTreePipeline.UEFIVendor = img.Platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.ISOLabel

	kernelOpts := []string{fmt.Sprintf("inst.stage2=hd:LABEL=%s", img.ISOLabel)}
	if img.ISORootKickstart {
		kernelOpts = append(kernelOpts, fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", img.ISOLabel, img.Kickstart.Path))
	}
	if img.OSCustomizations.FIPS {
		kernelOpts = append(kernelOpts, "fips=1")
	}
	kernelOpts = append(kernelOpts, img.AdditionalKernelOpts...)
	bootTreePipeline.KernelOpts = kernelOpts

	osPipeline := manifest.NewOS(buildPipeline, img.Platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.Workload = img.Workload

	// enable ISOLinux on x86_64 only
	isoLinuxEnabled := img.Platform.GetArch() == arch.ARCH_X86_64

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, anacondaPipeline, rootfsImagePipeline, bootTreePipeline)
	// TODO: the partition table is required - make it a ctor arg or set a default one in the pipeline
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.Release
	isoTreePipeline.Kickstart = img.Kickstart
	isoTreePipeline.PayloadPath = tarPath
	if img.ISORootKickstart {
		isoTreePipeline.Kickstart.Path = img.Kickstart.Path
	}

	isoTreePipeline.SquashfsCompression = img.SquashfsCompression

	isoTreePipeline.OSPipeline = osPipeline
	isoTreePipeline.KernelOpts = img.AdditionalKernelOpts
	if img.OSCustomizations.FIPS {
		isoTreePipeline.KernelOpts = append(isoTreePipeline.KernelOpts, "fips=1")
	}

	isoTreePipeline.ISOLinux = isoLinuxEnabled

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.ISOLabel)
	isoPipeline.SetFilename(img.Filename)
	isoPipeline.ISOLinux = isoLinuxEnabled

	artifact := isoPipeline.Export()

	return artifact, nil
}
