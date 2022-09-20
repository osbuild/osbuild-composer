package image

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/environment"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

type ImageInstaller struct {
	Base
	Platform          platform.Platform
	ExtraBasePackages rpmmd.PackageSet
	Users             []blueprint.UserCustomization
	Groups            []blueprint.GroupCustomization

	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment
	Workload         workload.Workload

	ISOLabelTempl string
	Product       string
	Variant       string
	OSName        string
	OSVersion     string
	Release       string

	Filename string
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
		repos, "kernel",
		img.Product,
		img.OSVersion)
	anacondaPipeline.ExtraPackages = img.ExtraBasePackages.Include
	anacondaPipeline.ExtraRepos = img.ExtraBasePackages.Repositories
	anacondaPipeline.Users = len(img.Users)+len(img.Groups) > 0
	anacondaPipeline.Variant = img.Variant
	anacondaPipeline.Biosdevname = (img.Platform.GetArch() == platform.ARCH_X86_64)
	anacondaPipeline.Checkpoint()

	osPipeline := manifest.NewOS(m, buildPipeline, img.Platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.Workload = img.Workload

	isoTreePipeline := manifest.NewISOTree(m,
		buildPipeline,
		anacondaPipeline,
		nil,
		osPipeline,
		img.ISOLabelTempl)
	isoTreePipeline.Release = img.Release
	isoTreePipeline.OSName = img.OSName
	isoTreePipeline.UEFIVendor = img.Platform.GetUEFIVendor()
	isoTreePipeline.Users = img.Users
	isoTreePipeline.Groups = img.Groups

	isoPipeline := manifest.NewISO(m, buildPipeline, isoTreePipeline)
	isoPipeline.Filename = img.Filename
	artifact := isoPipeline.Export()

	return artifact, nil
}
