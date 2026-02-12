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

type AnacondaContainerInstallerLegacy struct {
	Base
	AnacondaInstallerBase

	ExtraBasePackages rpmmd.PackageSet

	ContainerSource           container.SourceSpec
	ContainerRemoveSignatures bool

	// Locale for the installer. This should be set to the same locale as the
	// ISO OS payload, if known.
	Locale string

	// Filesystem type for the installed system as opposed to that of the ISO.
	InstallRootfsType disk.FSType
}

func NewAnacondaContainerInstallerLegacy(platform platform.Platform, filename string, container container.SourceSpec) *AnacondaContainerInstallerLegacy {
	return &AnacondaContainerInstallerLegacy{
		Base:            NewBase("container-installer", platform, filename),
		ContainerSource: container,
	}
}

func (img *AnacondaContainerInstallerLegacy) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {

	if img.BuildOptions == nil {
		img.BuildOptions = &manifest.BuildOptions{}
	}
	img.BuildOptions.ContainerBuildable = true
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, img.BuildOptions)
	buildPipeline.Checkpoint()

	anacondaPipeline := manifest.NewAnacondaInstaller(
		manifest.AnacondaInstallerTypePayload,
		buildPipeline,
		img.platform,
		repos,
		"kernel",
		img.InstallerCustomizations,
		img.ISOCustomizations,
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
	switch img.ISOCustomizations.RootfsType {
	case manifest.SquashfsExt4Rootfs:
		rootfsImagePipeline = manifest.NewISORootfsImg(buildPipeline, anacondaPipeline)
		rootfsImagePipeline.Size = 4 * datasizes.GibiByte
	default:
	}

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
	bootTreePipeline.Platform = img.platform
	bootTreePipeline.UEFIVendor = img.platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.ISOCustomizations.Label

	if img.Kickstart == nil {
		img.Kickstart = &kickstart.Options{}
	}
	if img.Kickstart.Path == "" {
		img.Kickstart.Path = osbuild.KickstartPathOSBuild
	}

	kernelOpts := []string{fmt.Sprintf("inst.stage2=hd:LABEL=%s", img.ISOCustomizations.Label), fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", img.ISOCustomizations.Label, img.Kickstart.Path)}
	if anacondaPipeline.InstallerCustomizations.FIPS {
		kernelOpts = append(kernelOpts, "fips=1")
	}
	kernelOpts = append(kernelOpts, img.InstallerCustomizations.KernelOptionsAppend...)
	bootTreePipeline.KernelOpts = kernelOpts

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, anacondaPipeline, rootfsImagePipeline, bootTreePipeline)
	initIsoTreePipeline(isoTreePipeline, &img.AnacondaInstallerBase, rng)
	// For ostree installers, always put the kickstart file in the root of the ISO
	isoTreePipeline.PayloadPath = "/container"
	isoTreePipeline.PayloadRemoveSignatures = img.ContainerRemoveSignatures
	isoTreePipeline.ContainerSource = &img.ContainerSource
	isoTreePipeline.InstallRootfsType = img.InstallRootfsType

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.ISOCustomizations)
	isoPipeline.SetFilename(img.filename)
	artifact := isoPipeline.Export()

	return artifact, nil
}
