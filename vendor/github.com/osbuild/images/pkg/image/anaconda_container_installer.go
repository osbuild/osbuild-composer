package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type AnacondaContainerInstaller struct {
	Base
	Platform          platform.Platform
	ExtraBasePackages rpmmd.PackageSet
	Users             []users.User
	Groups            []users.Group

	SquashfsCompression string

	ISOLabel  string
	Product   string
	Variant   string
	OSName    string
	Ref       string
	OSVersion string
	Release   string
	Preview   bool

	ContainerSource container.SourceSpec

	Filename string

	AdditionalDracutModules   []string
	AdditionalAnacondaModules []string
	AdditionalDrivers         []string
	FIPS                      bool

	// Kernel options that will be apended to the installed system
	// (not the iso)
	KickstartKernelOptionsAppend []string
	// Enable networking on on boot in the installed system
	KickstartNetworkOnBoot bool
}

func NewAnacondaContainerInstaller(container container.SourceSpec, ref string) *AnacondaContainerInstaller {
	return &AnacondaContainerInstaller{
		Base:            NewBase("container-installer"),
		ContainerSource: container,
		Ref:             ref,
	}
}

func (img *AnacondaContainerInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos, &manifest.BuildOptions{ContainerBuildable: true})
	buildPipeline.Checkpoint()

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

	// This is only built with ELN for now
	anacondaPipeline.UseRHELLoraxTemplates = true

	anacondaPipeline.ExtraPackages = img.ExtraBasePackages.Include
	anacondaPipeline.ExcludePackages = img.ExtraBasePackages.Exclude
	anacondaPipeline.ExtraRepos = img.ExtraBasePackages.Repositories
	anacondaPipeline.Users = img.Users
	anacondaPipeline.Groups = img.Groups
	anacondaPipeline.Variant = img.Variant
	anacondaPipeline.Biosdevname = (img.Platform.GetArch() == arch.ARCH_X86_64)
	anacondaPipeline.Checkpoint()
	anacondaPipeline.AdditionalDracutModules = img.AdditionalDracutModules
	anacondaPipeline.AdditionalAnacondaModules = img.AdditionalAnacondaModules
	if img.FIPS {
		anacondaPipeline.AdditionalAnacondaModules = append(
			anacondaPipeline.AdditionalAnacondaModules,
			"org.fedoraproject.Anaconda.Modules.Security",
		)
	}
	anacondaPipeline.AdditionalDrivers = img.AdditionalDrivers

	rootfsImagePipeline := manifest.NewISORootfsImg(buildPipeline, anacondaPipeline)
	rootfsImagePipeline.Size = 4 * common.GibiByte

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.Product, img.OSVersion)
	bootTreePipeline.Platform = img.Platform
	bootTreePipeline.UEFIVendor = img.Platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.ISOLabel

	kspath := osbuild.KickstartPathOSBuild
	bootTreePipeline.KernelOpts = []string{fmt.Sprintf("inst.stage2=hd:LABEL=%s", img.ISOLabel), fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", img.ISOLabel, kspath)}
	if img.FIPS {
		bootTreePipeline.KernelOpts = append(bootTreePipeline.KernelOpts, "fips=1")
	}

	// enable ISOLinux on x86_64 only
	isoLinuxEnabled := img.Platform.GetArch() == arch.ARCH_X86_64

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, anacondaPipeline, rootfsImagePipeline, bootTreePipeline)
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.Release
	isoTreePipeline.OSName = img.OSName
	isoTreePipeline.Users = img.Users
	isoTreePipeline.Groups = img.Groups
	isoTreePipeline.KickstartKernelOptionsAppend = img.KickstartKernelOptionsAppend

	isoTreePipeline.KickstartNetworkOnBoot = img.KickstartNetworkOnBoot

	isoTreePipeline.SquashfsCompression = img.SquashfsCompression

	// For ostree installers, always put the kickstart file in the root of the ISO
	isoTreePipeline.KSPath = kspath
	isoTreePipeline.PayloadPath = "/container"

	isoTreePipeline.ContainerSource = &img.ContainerSource
	isoTreePipeline.ISOLinux = isoLinuxEnabled
	if img.FIPS {
		isoTreePipeline.KernelOpts = append(isoTreePipeline.KernelOpts, "fips=1")
	}

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.ISOLabel)
	isoPipeline.SetFilename(img.Filename)
	isoPipeline.ISOLinux = isoLinuxEnabled
	artifact := isoPipeline.Export()

	return artifact, nil
}
