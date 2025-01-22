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
	Platform          platform.Platform
	ExtraBasePackages rpmmd.PackageSet

	Kickstart *kickstart.Options

	// Subscription options to include
	Subscription *subscription.ImageOptions

	RootfsCompression string
	RootfsType        manifest.RootfsType

	ISOLabel  string
	Product   string
	Variant   string
	OSVersion string
	Release   string
	Preview   bool

	Commit ostree.SourceSpec

	Filename string

	AdditionalDracutModules   []string
	AdditionalAnacondaModules []string
	DisabledAnacondaModules   []string
	AdditionalDrivers         []string
	FIPS                      bool

	// Uses the old, deprecated, Anaconda config option "kickstart-modules".
	// Only for RHEL 8.
	UseLegacyAnacondaConfig bool

	// Locale for the installer. This should be set to the same locale as the
	// ISO OS payload, if known.
	Locale string
}

func NewAnacondaOSTreeInstaller(commit ostree.SourceSpec) *AnacondaOSTreeInstaller {
	return &AnacondaOSTreeInstaller{
		Base:   NewBase("ostree-installer"),
		Commit: commit,
	}
}

func (img *AnacondaOSTreeInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos, nil)
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
	anacondaPipeline.Checkpoint()

	anacondaPipeline.UseLegacyAnacondaConfig = img.UseLegacyAnacondaConfig
	anacondaPipeline.AdditionalDracutModules = img.AdditionalDracutModules
	anacondaPipeline.AdditionalAnacondaModules = img.AdditionalAnacondaModules
	if img.FIPS {
		anacondaPipeline.AdditionalAnacondaModules = append(
			anacondaPipeline.AdditionalAnacondaModules,
			anaconda.ModuleSecurity,
		)
	}
	anacondaPipeline.DisabledAnacondaModules = img.DisabledAnacondaModules
	anacondaPipeline.AdditionalDrivers = img.AdditionalDrivers
	anacondaPipeline.Locale = img.Locale

	var rootfsImagePipeline *manifest.ISORootfsImg
	switch img.RootfsType {
	case manifest.SquashfsExt4Rootfs:
		rootfsImagePipeline = manifest.NewISORootfsImg(buildPipeline, anacondaPipeline)
		rootfsImagePipeline.Size = 4 * datasizes.GibiByte
	default:
	}

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.Product, img.OSVersion)
	bootTreePipeline.Platform = img.Platform
	bootTreePipeline.UEFIVendor = img.Platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.ISOLabel

	if img.Kickstart == nil || img.Kickstart.OSTree == nil {
		return nil, fmt.Errorf("kickstart options not set for ostree installer")
	}
	if img.Kickstart.Path == "" {
		img.Kickstart.Path = osbuild.KickstartPathOSBuild
	}
	bootTreePipeline.KernelOpts = []string{fmt.Sprintf("inst.stage2=hd:LABEL=%s", img.ISOLabel), fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", img.ISOLabel, img.Kickstart.Path)}
	if img.FIPS {
		bootTreePipeline.KernelOpts = append(bootTreePipeline.KernelOpts, "fips=1")
	}

	// enable ISOLinux on x86_64 only
	isoLinuxEnabled := img.Platform.GetArch() == arch.ARCH_X86_64

	var subscriptionPipeline *manifest.Subscription
	if img.Subscription != nil {
		// pipeline that will create subscription service and key file to be copied out
		subscriptionPipeline = manifest.NewSubscription(buildPipeline, img.Subscription)
	}

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, anacondaPipeline, rootfsImagePipeline, bootTreePipeline)
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.Release
	isoTreePipeline.Kickstart = img.Kickstart
	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.RootfsType

	isoTreePipeline.PayloadPath = "/ostree/repo"

	isoTreePipeline.OSTreeCommitSource = &img.Commit
	isoTreePipeline.ISOLinux = isoLinuxEnabled
	if img.FIPS {
		isoTreePipeline.KernelOpts = append(isoTreePipeline.KernelOpts, "fips=1")
	}
	isoTreePipeline.SubscriptionPipeline = subscriptionPipeline

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.ISOLabel)
	isoPipeline.SetFilename(img.Filename)
	isoPipeline.ISOLinux = isoLinuxEnabled
	artifact := isoPipeline.Export()

	return artifact, nil
}
