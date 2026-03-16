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
	AnacondaInstallerBase

	Environment       environment.Environment
	ExtraBasePackages rpmmd.PackageSet

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
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, img.BuildOptions)
	buildPipeline.Checkpoint()

	livePipeline := manifest.NewAnacondaInstaller(
		manifest.AnacondaInstallerTypeLive,
		buildPipeline,
		img.platform,
		repos,
		"kernel",
		img.InstallerCustomizations,
		img.ISOCustomizations,
	)

	livePipeline.ExtraPackages = img.ExtraBasePackages.Include
	livePipeline.ExcludePackages = img.ExtraBasePackages.Exclude

	livePipeline.Biosdevname = (img.platform.GetArch() == arch.ARCH_X86_64)
	livePipeline.Locale = img.Locale

	// The live installer has SELinux enabled and targeted
	livePipeline.SELinux = "targeted"

	livePipeline.Checkpoint()

	var rootfsImagePipeline *manifest.ISORootfsImg
	switch img.ISOCustomizations.RootfsType {
	case manifest.SquashfsExt4Rootfs:
		rootfsImagePipeline = manifest.NewISORootfsImg(buildPipeline, livePipeline)
		rootfsImagePipeline.Size = 8 * datasizes.GibiByte
	default:
	}

	// Setup the kernel options for the live iso
	kernelOpts := []string{
		fmt.Sprintf("root=live:CDLABEL=%s", img.ISOCustomizations.Label),
		"rd.live.image",
		"quiet",
		"rhgb",
	}

	kernelOpts = append(kernelOpts, img.InstallerCustomizations.KernelOptionsAppend...)

	// Setup the bootloaders
	bootloaders := img.Bootloaders(buildPipeline, img.platform, kernelOpts)

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(
		buildPipeline,
		livePipeline,
		rootfsImagePipeline,
		bootloaders,
		img.InstallerCustomizations,
		img.ISOCustomizations,
	)
	initIsoTreePipeline(isoTreePipeline, &img.AnacondaInstallerBase, rng)

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.ISOCustomizations)
	isoPipeline.SetFilename(img.filename)

	artifact := isoPipeline.Export()

	return artifact, nil
}
