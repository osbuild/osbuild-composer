package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/fsnode"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
	"github.com/osbuild/osbuild-composer/internal/users"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

type OSTreeImage struct {
	Base

	Platform       platform.Platform
	Workload       workload.Workload
	PartitionTable *disk.PartitionTable

	Users  []users.User
	Groups []users.Group

	CommitSource ostree.SourceSpec

	SysrootReadOnly bool

	Remote *ostree.Remote
	OSName string

	KernelOptionsAppend []string
	Keyboard            string
	Locale              string

	Filename string

	Ignition         bool
	IgnitionPlatform string
	Compression      string

	Directories []*fsnode.Directory
	Files       []*fsnode.File
}

func NewOSTreeImage(commit ostree.SourceSpec) *OSTreeImage {
	return &OSTreeImage{
		Base:         NewBase("ostree-raw-image"),
		CommitSource: commit,
	}
}

func ostreeDeploymentPipeline(img *OSTreeImage, m *manifest.Manifest, buildPipeline *manifest.Build) *manifest.OSTreeDeployment {
	osPipeline := manifest.NewOSTreeDeployment(m, buildPipeline, img.CommitSource, img.OSName, img.Ignition, img.IgnitionPlatform, img.Platform)
	osPipeline.PartitionTable = img.PartitionTable
	osPipeline.Remote = img.Remote
	osPipeline.KernelOptionsAppend = img.KernelOptionsAppend
	osPipeline.Keyboard = img.Keyboard
	osPipeline.Locale = img.Locale
	osPipeline.Users = img.Users
	osPipeline.Groups = img.Groups
	osPipeline.SysrootReadOnly = img.SysrootReadOnly
	osPipeline.Directories = img.Directories
	osPipeline.Files = img.Files

	// other image types (e.g. live) pass the workload to the pipeline.
	osPipeline.EnabledServices = img.Workload.GetServices()
	osPipeline.DisabledServices = img.Workload.GetDisabledServices()

	return osPipeline
}

func (img *OSTreeImage) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos)
	buildPipeline.Checkpoint()

	osPipeline := ostreeDeploymentPipeline(img, m, buildPipeline)
	rawImgPipeline := manifest.NewRawOStreeImage(m, buildPipeline, img.Platform, osPipeline)

	var art *artifact.Artifact
	switch img.Platform.GetImageFormat() {
	case platform.FORMAT_RAW:
		// check for compression
		switch img.Compression {
		case "xz":
			// compress image with xz
			xzPipeline := manifest.NewXZ(m, buildPipeline, rawImgPipeline)
			// output of the final xz pipeline should be the filename specified for the image
			xzPipeline.Filename = img.Filename
			art = xzPipeline.Export()
		case "":
			// no compression: set the name of the raw pipeline to the filename specified for the image
			rawImgPipeline.Filename = img.Filename
			art = rawImgPipeline.Export()
		default:
			panic(fmt.Sprintf("unsupported compression type %q on %q", img.Compression, img.name))
		}
	case platform.FORMAT_QCOW2:
		// convert raw image to qcow2 (no compression supported)
		qcow2Pipeline := manifest.NewQCOW2(m, buildPipeline, rawImgPipeline.GetManifest(), rawImgPipeline.Name(), rawImgPipeline.Filename)
		qcow2Pipeline.Compat = img.Platform.GetQCOW2Compat()
		// output of the final qcow2 pipeline should be the filename specified for the image
		qcow2Pipeline.Filename = img.Filename
		art = qcow2Pipeline.Export()
	default:
		panic("invalid image format for image kind")
	}

	return art, nil
}
