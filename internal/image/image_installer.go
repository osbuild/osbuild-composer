package image

import (
	"fmt"
	"math/rand"
	"path/filepath"

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

const kspath = "/osbuild.ks"

type ImageInstaller struct {
	Base
	Platform         platform.Platform
	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment
	Workload         workload.Workload

	ExtraBasePackages rpmmd.PackageSet
	Users             []users.User
	Groups            []users.Group

	// If set, the kickstart file will be added to the bootiso-tree as
	// /osbuild.ks, otherwise any kickstart options will be configured in the
	// default /usr/share/anaconda/interactive-defaults.ks in the rootfs.
	ISORootKickstart bool

	SquashfsCompression string

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

	anacondaPipeline.ExtraPackages = img.ExtraBasePackages.Include
	anacondaPipeline.ExtraRepos = img.ExtraBasePackages.Repositories
	anacondaPipeline.Users = img.Users
	anacondaPipeline.Groups = img.Groups
	anacondaPipeline.Variant = img.Variant
	anacondaPipeline.Biosdevname = (img.Platform.GetArch() == platform.ARCH_X86_64)
	anacondaPipeline.AdditionalModules = img.AdditionalAnacondaModules

	tarPath := "/liveimg.tar.gz"

	if !img.ISORootKickstart {
		payloadPath := filepath.Join("/run/install/repo/", tarPath)
		anacondaPipeline.InteractiveDefaults = manifest.NewAnacondaInteractiveDefaults(fmt.Sprintf("file://%s", payloadPath))
	}

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
	if img.ISORootKickstart {
		bootTreePipeline.KSPath = kspath
	}

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
	isoTreePipeline.PayloadPath = tarPath
	if img.ISORootKickstart {
		isoTreePipeline.KSPath = kspath
	}

	isoTreePipeline.SquashfsCompression = img.SquashfsCompression

	isoTreePipeline.OSPipeline = osPipeline
	isoTreePipeline.KernelOpts = img.AdditionalKernelOpts

	isoPipeline := manifest.NewISO(m, buildPipeline, isoTreePipeline)
	isoPipeline.Filename = img.Filename
	isoPipeline.ISOLinux = img.Platform.GetArch() == platform.ARCH_X86_64

	artifact := isoPipeline.Export()

	return artifact, nil
}
