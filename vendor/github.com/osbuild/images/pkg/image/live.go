package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type LiveImage struct {
	Base
	Platform    platform.Platform
	Environment environment.Environment
	Workload    workload.Workload

	ExtraBasePackages rpmmd.PackageSet

	ISOLabelTempl string
	Product       string
	Variant       string
	OSName        string
	OSVersion     string
	Release       string

	Filename string

	AdditionalKernelOpts []string
	OSCustomizations     manifest.OSCustomizations
}

func NewLiveImage() *LiveImage {
	return &LiveImage{
		Base: NewBase("live-media"),
	}
}

func (img *LiveImage) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(m, buildPipeline, img.Platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.Workload = img.Workload

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

	rootfsImagePipeline := manifest.NewISORootfsImg(m, buildPipeline, osPipeline)
	rootfsImagePipeline.Size = 8 * common.GibiByte

	bootTreePipeline := manifest.NewEFIBootTree(m, buildPipeline, img.Product, img.OSVersion)
	bootTreePipeline.Platform = img.Platform
	bootTreePipeline.UEFIVendor = img.Platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = isoLabel

	kernelOpts := []string{
		fmt.Sprintf("root=live:CDLABEL=%s", isoLabel),
		"rd.live.image",
		"quiet",
		"rhgb",
	}

	kernelOpts = append(kernelOpts, img.AdditionalKernelOpts...)

	bootTreePipeline.KernelOpts = kernelOpts

	// enable ISOLinux on x86_64 only
	isoLinuxEnabled := img.Platform.GetArch() == platform.ARCH_X86_64

	isoTreePipeline := manifest.NewLiveTree(m,
		buildPipeline,
		osPipeline,
		rootfsImagePipeline,
		bootTreePipeline,
		isoLabel)
	isoTreePipeline.PartitionTable = rootfsPartitionTable
	isoTreePipeline.Release = img.Release
	isoTreePipeline.OSName = img.OSName

	isoTreePipeline.KernelOpts = kernelOpts
	isoTreePipeline.ISOLinux = isoLinuxEnabled

	isoTreePipeline.Product = img.Product
	isoTreePipeline.Version = img.OSVersion

	isoPipeline := manifest.NewISO(m, buildPipeline, isoTreePipeline, isoLabel)
	isoPipeline.Filename = img.Filename
	isoPipeline.ISOLinux = isoLinuxEnabled

	artifact := isoPipeline.Export()

	return artifact, nil
}
