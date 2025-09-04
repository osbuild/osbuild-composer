package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/environment"
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
	InstallerCustomizations manifest.InstallerCustomizations
	Environment             environment.Environment

	ExtraBasePackages rpmmd.PackageSet

	RootfsCompression string

	// Locale for the installer. This should be set to the same locale as the
	// ISO OS payload, if known.
	Locale string

	AdditionalDracutModules []string
	AdditionalDrivers       []string
}

func NewAnacondaLiveInstaller(platform platform.Platform, filename string) *AnacondaLiveInstaller {
	return &AnacondaLiveInstaller{
		Base: NewBase("live-installer", platform, filename),
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
		img.platform,
		repos,
		"kernel",
		img.InstallerCustomizations,
	)

	livePipeline.ExtraPackages = img.ExtraBasePackages.Include
	livePipeline.ExcludePackages = img.ExtraBasePackages.Exclude

	livePipeline.Biosdevname = (img.platform.GetArch() == arch.ARCH_X86_64)
	livePipeline.Locale = img.Locale

	// The live installer has SELinux enabled and targeted
	livePipeline.SELinux = "targeted"

	livePipeline.Checkpoint()

	var rootfsImagePipeline *manifest.ISORootfsImg
	switch img.InstallerCustomizations.ISORootfsType {
	case manifest.SquashfsExt4Rootfs:
		rootfsImagePipeline = manifest.NewISORootfsImg(buildPipeline, livePipeline)
		rootfsImagePipeline.Size = 8 * datasizes.GibiByte
	default:
	}

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
	bootTreePipeline.Platform = img.platform
	bootTreePipeline.UEFIVendor = img.platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.InstallerCustomizations.ISOLabel
	bootTreePipeline.DefaultMenu = img.InstallerCustomizations.DefaultMenu

	kernelOpts := []string{
		fmt.Sprintf("root=live:CDLABEL=%s", img.InstallerCustomizations.ISOLabel),
		"rd.live.image",
		"quiet",
		"rhgb",
	}

	kernelOpts = append(kernelOpts, img.InstallerCustomizations.AdditionalKernelOpts...)

	bootTreePipeline.KernelOpts = kernelOpts

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, livePipeline, rootfsImagePipeline, bootTreePipeline)
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.InstallerCustomizations.Release

	isoTreePipeline.KernelOpts = kernelOpts
	isoTreePipeline.ISOBoot = img.InstallerCustomizations.ISOBoot

	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.InstallerCustomizations.ISORootfsType

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.InstallerCustomizations.ISOLabel)
	isoPipeline.SetFilename(img.filename)
	isoPipeline.ISOBoot = img.InstallerCustomizations.ISOBoot

	artifact := isoPipeline.Export()

	return artifact, nil
}
