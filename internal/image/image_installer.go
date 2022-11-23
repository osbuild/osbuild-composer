package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/environment"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
	"github.com/osbuild/osbuild-composer/internal/users"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

type ImageInstaller struct {
	Base
	Platform         platform.Platform
	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment
	Workload         workload.Workload

	ExtraBasePackages rpmmd.PackageSet
	Users             []users.User
	Groups            []users.Group

	ISOLabelTempl string
	Product       string
	Variant       string
	OSName        string
	OSVersion     string
	Release       string

	Filename string

	AdditionalKernelOpts      []string
	AdditionalAnacondaModules []string
}

func NewImageInstaller() *ImageInstaller {
	return &ImageInstaller{
		Base: NewBase("image-installer"),
	}
}

func (img *ImageInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos)
	buildPipeline.Checkpoint()

	anacondaPipeline := manifest.NewAnaconda(m,
		buildPipeline,
		img.Platform,
		repos,
		"kernel",
		img.Product,
		img.OSVersion)

	interactiveDefaults := manifest.NewAnacondaInteractiveDefaults(
		"file:///run/install/repo/liveimg.tar.gz",
	)

	anacondaPipeline.ExtraPackages = img.ExtraBasePackages.Include
	anacondaPipeline.ExtraRepos = img.ExtraBasePackages.Repositories
	anacondaPipeline.Users = len(img.Users)+len(img.Groups) > 0
	anacondaPipeline.Variant = img.Variant
	anacondaPipeline.Biosdevname = (img.Platform.GetArch() == platform.ARCH_X86_64)
	anacondaPipeline.InteractiveDefaults = interactiveDefaults
	anacondaPipeline.AdditionalModules = img.AdditionalAnacondaModules

	anacondaPipeline.Checkpoint()

	rootfsPartitionTable := &disk.PartitionTable{
		Size: 20 * common.MebiByte,
		Partitions: []disk.Partition{
			{
				Start: 0,
				Size:  20 * common.MebiByte,
				Payload: &disk.Filesystem{
					Type:       "vfat",
					Mountpoint: "/",
					UUID:       disk.NewVolIDFromRand(rng),
				},
			},
		},
	}

	// TODO: replace isoLabelTmpl with more high-level properties
	isoLabel := fmt.Sprintf(img.ISOLabelTempl, img.Platform.GetArch())

	rootfsImagePipeline := manifest.NewISORootfsImg(m, buildPipeline, anacondaPipeline)
	rootfsImagePipeline.Size = 4 * common.GibiByte

	bootTreePipeline := manifest.NewEFIBootTree(m, buildPipeline, anacondaPipeline)
	bootTreePipeline.Platform = img.Platform
	bootTreePipeline.UEFIVendor = img.Platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = isoLabel
	bootTreePipeline.KernelOpts = img.AdditionalKernelOpts

	osPipeline := manifest.NewOS(m, buildPipeline, img.Platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.Workload = img.Workload

	isoTreePipeline := manifest.NewISOTree(m,
		buildPipeline,
		anacondaPipeline,
		rootfsImagePipeline,
		bootTreePipeline,
		isoLabel)
	isoTreePipeline.PartitionTable = rootfsPartitionTable
	isoTreePipeline.Release = img.Release
	isoTreePipeline.OSName = img.OSName
	isoTreePipeline.Users = img.Users
	isoTreePipeline.Groups = img.Groups

	isoTreePipeline.OSPipeline = osPipeline
	isoTreePipeline.KernelOpts = img.AdditionalKernelOpts

	isoPipeline := manifest.NewISO(m, buildPipeline, isoTreePipeline)
	isoPipeline.Filename = img.Filename
	isoPipeline.ISOLinux = true

	artifact := isoPipeline.Export()

	return artifact, nil
}
