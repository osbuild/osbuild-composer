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
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

func efiBootPartitionTable(rng *rand.Rand) *disk.PartitionTable {
	efibootImageSize := datasizes.Size(20 * datasizes.MebiByte)
	return &disk.PartitionTable{
		Size: efibootImageSize,
		Partitions: []disk.Partition{
			{
				Start: 0,
				Size:  efibootImageSize,
				Payload: &disk.Filesystem{
					Type:       "vfat",
					Mountpoint: "/",
					UUID:       disk.NewVolIDFromRand(rng),
				},
			},
		},
	}
}

type AnacondaTarInstaller struct {
	Base
	AnacondaInstallerBase

	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment

	ExtraBasePackages rpmmd.PackageSet
}

func NewAnacondaTarInstaller(platform platform.Platform, filename string) *AnacondaTarInstaller {
	return &AnacondaTarInstaller{
		Base: NewBase("image-installer", platform, filename),
	}
}

func (img *AnacondaTarInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, img.BuildOptions)
	buildPipeline.Checkpoint()

	if img.Kickstart == nil {
		img.Kickstart = &kickstart.Options{}
	}

	if img.Kickstart.Path == "" {
		img.Kickstart.Path = osbuild.KickstartPathOSBuild
	}

	img.InstallerCustomizations.Payload.Path = "/liveimg.tar.gz"

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
	if img.Kickstart != nil {
		anacondaPipeline.InteractiveDefaultsKickstart = &kickstart.Options{
			Users:  img.Kickstart.Users,
			Groups: img.Kickstart.Groups,
		}
	}
	anacondaPipeline.Biosdevname = (img.platform.GetArch() == arch.ARCH_X86_64)

	if img.OSCustomizations.FIPS {
		anacondaPipeline.InstallerCustomizations.EnabledAnacondaModules = append(
			anacondaPipeline.InstallerCustomizations.EnabledAnacondaModules,
			anaconda.ModuleSecurity,
		)
	}

	anacondaPipeline.Locale = img.OSCustomizations.Language

	anacondaPipeline.Checkpoint()

	var rootfsImagePipeline *manifest.ISORootfsImg
	switch img.ISOCustomizations.RootfsType {
	case manifest.SquashfsExt4Rootfs:
		rootfsImagePipeline = manifest.NewISORootfsImg(buildPipeline, anacondaPipeline)
		rootfsImagePipeline.Size = 5 * datasizes.GibiByte
	default:
	}

	// Setup the kernel options for the tar installer
	kernelOpts := []string{
		fmt.Sprintf("inst.stage2=hd:LABEL=%s", img.ISOCustomizations.Label),
		fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", img.ISOCustomizations.Label, img.Kickstart.Path),
	}

	if img.OSCustomizations.FIPS {
		kernelOpts = append(kernelOpts, "fips=1")
	}
	kernelOpts = append(kernelOpts, img.InstallerCustomizations.KernelOptionsAppend...)

	// Setup the bootloaders
	bootloaders := img.Bootloaders(buildPipeline, img.platform, kernelOpts)

	osPipeline := manifest.NewOS(buildPipeline, img.platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(
		buildPipeline,
		anacondaPipeline,
		rootfsImagePipeline,
		bootloaders,
		img.InstallerCustomizations,
		img.ISOCustomizations,
	)
	initIsoTreePipeline(isoTreePipeline, &img.AnacondaInstallerBase, rng)

	isoTreePipeline.OSPipeline = osPipeline

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.ISOCustomizations)
	isoPipeline.SetFilename(img.filename)

	artifact := isoPipeline.Export()

	return artifact, nil
}
