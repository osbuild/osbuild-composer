package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type AnacondaLiveInstaller struct {
	Base
	Platform    platform.Platform
	Environment environment.Environment
	Workload    workload.Workload

	ExtraBasePackages rpmmd.PackageSet

	RootfsCompression string
	RootfsType        manifest.RootfsType
	ISOBoot           manifest.ISOBootType

	ISOLabel  string
	Product   string
	Variant   string
	OSVersion string
	Release   string
	Preview   bool

	Filename string

	AdditionalKernelOpts []string

	// Locale for the installer. This should be set to the same locale as the
	// ISO OS payload, if known.
	Locale string

	AdditionalDracutModules []string
	AdditionalDrivers       []string
}

func NewAnacondaLiveInstaller() *AnacondaLiveInstaller {
	return &AnacondaLiveInstaller{
		Base: NewBase("live-installer"),
	}
}

func (img *AnacondaLiveInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	livePipeline := manifest.NewAnacondaInstaller(
		manifest.AnacondaInstallerTypeLive,
		buildPipeline,
		img.Platform,
		repos,
		"kernel",
		img.Product,
		img.OSVersion,
		img.Preview,
	)

	livePipeline.ExtraPackages = img.ExtraBasePackages.Include
	livePipeline.ExcludePackages = img.ExtraBasePackages.Exclude

	livePipeline.Variant = img.Variant
	livePipeline.Biosdevname = (img.Platform.GetArch() == arch.ARCH_X86_64)
	livePipeline.Locale = img.Locale

	// The live installer has SElinux enabled and targeted
	livePipeline.SElinux = "targeted"

	livePipeline.Checkpoint()

	var rootfsImagePipeline *manifest.ISORootfsImg
	switch img.RootfsType {
	case manifest.SquashfsExt4Rootfs:
		rootfsImagePipeline = manifest.NewISORootfsImg(buildPipeline, livePipeline)
		rootfsImagePipeline.Size = 8 * datasizes.GibiByte
	default:
	}

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.Product, img.OSVersion)
	bootTreePipeline.Platform = img.Platform
	bootTreePipeline.UEFIVendor = img.Platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.ISOLabel

	kernelOpts := []string{
		fmt.Sprintf("root=live:CDLABEL=%s", img.ISOLabel),
		"rd.live.image",
		"quiet",
		"rhgb",
	}

	kernelOpts = append(kernelOpts, img.AdditionalKernelOpts...)

	bootTreePipeline.KernelOpts = kernelOpts

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, livePipeline, rootfsImagePipeline, bootTreePipeline)
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.Release

	isoTreePipeline.KernelOpts = kernelOpts
	isoTreePipeline.ISOBoot = img.ISOBoot

	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.RootfsType

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.ISOLabel)
	isoPipeline.SetFilename(img.Filename)
	isoPipeline.ISOBoot = img.ISOBoot

	artifact := isoPipeline.Export()

	return artifact, nil
}
