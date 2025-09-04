package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type AnacondaNetInstaller struct {
	Base
	InstallerCustomizations manifest.InstallerCustomizations
	Environment             environment.Environment

	ExtraBasePackages rpmmd.PackageSet

	RootfsCompression string

	Language string
}

func NewAnacondaNetInstaller(platform platform.Platform, filename string) *AnacondaNetInstaller {
	return &AnacondaNetInstaller{
		Base: NewBase("netinst", platform, filename),
	}
}

func (img *AnacondaNetInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	installerType := manifest.AnacondaInstallerTypeNetinst
	anacondaPipeline := manifest.NewAnacondaInstaller(
		installerType,
		buildPipeline,
		img.platform,
		repos,
		"kernel",
		img.InstallerCustomizations,
	)

	anacondaPipeline.ExtraPackages = img.ExtraBasePackages.Include
	anacondaPipeline.ExcludePackages = img.ExtraBasePackages.Exclude
	anacondaPipeline.ExtraRepos = img.ExtraBasePackages.Repositories
	anacondaPipeline.Biosdevname = (img.platform.GetArch() == arch.ARCH_X86_64)

	if anacondaPipeline.InstallerCustomizations.FIPS {
		anacondaPipeline.InstallerCustomizations.EnabledAnacondaModules = append(
			anacondaPipeline.InstallerCustomizations.EnabledAnacondaModules,
			anaconda.ModuleSecurity,
		)
	}

	anacondaPipeline.Locale = img.Language

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

	kernelOpts := []string{fmt.Sprintf("inst.stage2=hd:LABEL=%s", img.InstallerCustomizations.ISOLabel)}
	if anacondaPipeline.InstallerCustomizations.FIPS {
		kernelOpts = append(kernelOpts, "fips=1")
	}
	kernelOpts = append(kernelOpts, img.InstallerCustomizations.AdditionalKernelOpts...)
	bootTreePipeline.KernelOpts = kernelOpts

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, anacondaPipeline, rootfsImagePipeline, bootTreePipeline)
	// TODO: the partition table is required - make it a ctor arg or set a default one in the pipeline
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.InstallerCustomizations.Release

	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.InstallerCustomizations.ISORootfsType

	isoTreePipeline.KernelOpts = img.InstallerCustomizations.AdditionalKernelOpts
	if anacondaPipeline.InstallerCustomizations.FIPS {
		isoTreePipeline.KernelOpts = append(isoTreePipeline.KernelOpts, "fips=1")
	}

	isoTreePipeline.ISOBoot = img.InstallerCustomizations.ISOBoot

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.InstallerCustomizations.ISOLabel)
	isoPipeline.SetFilename(img.filename)
	isoPipeline.ISOBoot = img.InstallerCustomizations.ISOBoot

	artifact := isoPipeline.Export()

	return artifact, nil
}
