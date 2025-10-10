package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/container"
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

type AnacondaContainerInstaller struct {
	Base

	InstallerCustomizations manifest.InstallerCustomizations
	ExtraBasePackages       rpmmd.PackageSet

	RootfsCompression string

	Ref string

	ContainerSource           container.SourceSpec
	ContainerRemoveSignatures bool

	Kickstart *kickstart.Options

	// Locale for the installer. This should be set to the same locale as the
	// ISO OS payload, if known.
	Locale string

	// Filesystem type for the installed system as opposed to that of the ISO.
	InstallRootfsType disk.FSType
}

func NewAnacondaContainerInstaller(platform platform.Platform, filename string, container container.SourceSpec, ref string) *AnacondaContainerInstaller {
	return &AnacondaContainerInstaller{
		Base:            NewBase("container-installer", platform, filename),
		ContainerSource: container,
		Ref:             ref,
	}
}

func (img *AnacondaContainerInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, &manifest.BuildOptions{ContainerBuildable: true})
	buildPipeline.Checkpoint()

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
	anacondaPipeline.Biosdevname = (img.platform.GetArch() == arch.ARCH_X86_64)
	anacondaPipeline.Checkpoint()

	if anacondaPipeline.InstallerCustomizations.FIPS {
		anacondaPipeline.InstallerCustomizations.EnabledAnacondaModules = append(
			anacondaPipeline.InstallerCustomizations.EnabledAnacondaModules,
			anaconda.ModuleSecurity,
		)
	}

	anacondaPipeline.Locale = img.Locale

	var rootfsImagePipeline *manifest.ISORootfsImg
	switch img.InstallerCustomizations.ISORootfsType {
	case manifest.SquashfsExt4Rootfs:
		rootfsImagePipeline = manifest.NewISORootfsImg(buildPipeline, anacondaPipeline)
		rootfsImagePipeline.Size = 4 * datasizes.GibiByte
	default:
	}

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
	bootTreePipeline.Platform = img.platform
	bootTreePipeline.UEFIVendor = img.platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.InstallerCustomizations.ISOLabel

	if img.Kickstart == nil {
		img.Kickstart = &kickstart.Options{}
	}
	if img.Kickstart.Path == "" {
		img.Kickstart.Path = osbuild.KickstartPathOSBuild
	}

	kernelOpts := []string{fmt.Sprintf("inst.stage2=hd:LABEL=%s", img.InstallerCustomizations.ISOLabel), fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", img.InstallerCustomizations.ISOLabel, img.Kickstart.Path)}
	if anacondaPipeline.InstallerCustomizations.FIPS {
		kernelOpts = append(kernelOpts, "fips=1")
	}
	kernelOpts = append(kernelOpts, img.InstallerCustomizations.KernelOptionsAppend...)
	bootTreePipeline.KernelOpts = kernelOpts

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, anacondaPipeline, rootfsImagePipeline, bootTreePipeline)
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.InstallerCustomizations.Release
	isoTreePipeline.Kickstart = img.Kickstart

	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.InstallerCustomizations.ISORootfsType

	// For ostree installers, always put the kickstart file in the root of the ISO
	isoTreePipeline.PayloadPath = "/container"
	isoTreePipeline.PayloadRemoveSignatures = img.ContainerRemoveSignatures

	isoTreePipeline.ContainerSource = &img.ContainerSource
	isoTreePipeline.ISOBoot = img.InstallerCustomizations.ISOBoot
	if anacondaPipeline.InstallerCustomizations.FIPS {
		isoTreePipeline.KernelOpts = append(isoTreePipeline.KernelOpts, "fips=1")
	}

	isoTreePipeline.InstallRootfsType = img.InstallRootfsType

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.InstallerCustomizations.ISOLabel)
	isoPipeline.SetFilename(img.filename)
	isoPipeline.ISOBoot = img.InstallerCustomizations.ISOBoot
	artifact := isoPipeline.Export()

	return artifact, nil
}
