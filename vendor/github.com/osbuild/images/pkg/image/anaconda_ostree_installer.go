package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type AnacondaOSTreeInstaller struct {
	Base
	InstallerCustomizations manifest.InstallerCustomizations
	ExtraBasePackages       rpmmd.PackageSet

	Kickstart *kickstart.Options

	// Subscription options to include
	Subscription *subscription.ImageOptions

	RootfsCompression string

	Commit ostree.SourceSpec

	// Locale for the installer. This should be set to the same locale as the
	// ISO OS payload, if known.
	Locale string
}

func NewAnacondaOSTreeInstaller(platform platform.Platform, filename string, commit ostree.SourceSpec) *AnacondaOSTreeInstaller {
	return &AnacondaOSTreeInstaller{
		Base:   NewBase("ostree-installer", platform, filename),
		Commit: commit,
	}
}

func (img *AnacondaOSTreeInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, nil)
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
	if img.Kickstart != nil {
		anacondaPipeline.InteractiveDefaultsKickstart = &kickstart.Options{
			Users:  img.Kickstart.Users,
			Groups: img.Kickstart.Groups,
		}
	}
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
	bootTreePipeline.DefaultMenu = img.InstallerCustomizations.DefaultMenu

	if img.Kickstart == nil || img.Kickstart.OSTree == nil {
		return nil, fmt.Errorf("kickstart options not set for ostree installer")
	}
	if img.Kickstart.Path == "" {
		img.Kickstart.Path = osbuild.KickstartPathOSBuild
	}
	bootTreePipeline.KernelOpts = []string{fmt.Sprintf("inst.stage2=hd:LABEL=%s", img.InstallerCustomizations.ISOLabel), fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", img.InstallerCustomizations.ISOLabel, img.Kickstart.Path)}
	if anacondaPipeline.InstallerCustomizations.FIPS {
		bootTreePipeline.KernelOpts = append(bootTreePipeline.KernelOpts, "fips=1")
	}

	var subscriptionPipeline *manifest.Subscription
	if img.Subscription != nil {
		// pipeline that will create subscription service and key file to be copied out
		subscriptionPipeline = manifest.NewSubscription(buildPipeline, img.Subscription)
	}

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, anacondaPipeline, rootfsImagePipeline, bootTreePipeline)
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.InstallerCustomizations.Release
	isoTreePipeline.Kickstart = img.Kickstart
	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.InstallerCustomizations.ISORootfsType

	isoTreePipeline.PayloadPath = "/ostree/repo"

	isoTreePipeline.OSTreeCommitSource = &img.Commit
	isoTreePipeline.ISOBoot = img.InstallerCustomizations.ISOBoot
	if anacondaPipeline.InstallerCustomizations.FIPS {
		isoTreePipeline.KernelOpts = append(isoTreePipeline.KernelOpts, "fips=1")
	}
	isoTreePipeline.SubscriptionPipeline = subscriptionPipeline

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.InstallerCustomizations.ISOLabel)
	isoPipeline.SetFilename(img.filename)
	isoPipeline.ISOBoot = img.InstallerCustomizations.ISOBoot
	artifact := isoPipeline.Export()

	return artifact, nil
}
