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
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type AnacondaNetInstaller struct {
	Base
	AnacondaInstallerBase

	Environment environment.Environment

	ExtraBasePackages rpmmd.PackageSet

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
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, img.BuildOptions)
	buildPipeline.Checkpoint()

	if img.Kickstart != nil && img.Kickstart.Path == "" {
		img.Kickstart.Path = osbuild.KickstartPathOSBuild
	}

	installerType := manifest.AnacondaInstallerTypeNetinst
	anacondaPipeline := manifest.NewAnacondaInstaller(
		installerType,
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
	if img.Kickstart != nil {
		anacondaPipeline.InteractiveDefaultsKickstart = &kickstart.Options{
			Users:  img.Kickstart.Users,
			Groups: img.Kickstart.Groups,
		}
	}
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
	switch img.ISOCustomizations.RootfsType {
	case manifest.SquashfsExt4Rootfs:
		rootfsImagePipeline = manifest.NewISORootfsImg(buildPipeline, anacondaPipeline)
		rootfsImagePipeline.Size = 5 * datasizes.GibiByte
	default:
	}

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
	bootTreePipeline.Platform = img.platform
	bootTreePipeline.UEFIVendor = img.platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.ISOCustomizations.Label
	bootTreePipeline.DefaultMenu = img.InstallerCustomizations.DefaultMenu

	kernelOpts := []string{
		fmt.Sprintf("inst.stage2=hd:LABEL=%s", img.ISOCustomizations.Label),
	}

	if img.Kickstart != nil {
		kernelOpts = append(kernelOpts, fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", img.ISOCustomizations.Label, img.Kickstart.Path))
	}

	if anacondaPipeline.InstallerCustomizations.FIPS {
		kernelOpts = append(kernelOpts, "fips=1")
	}
	kernelOpts = append(kernelOpts, img.InstallerCustomizations.KernelOptionsAppend...)
	bootTreePipeline.KernelOpts = kernelOpts

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, anacondaPipeline, rootfsImagePipeline, bootTreePipeline)
	initIsoTreePipeline(isoTreePipeline, &img.AnacondaInstallerBase, rng)

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.ISOCustomizations)
	isoPipeline.SetFilename(img.filename)

	artifact := isoPipeline.Export()

	return artifact, nil
}
