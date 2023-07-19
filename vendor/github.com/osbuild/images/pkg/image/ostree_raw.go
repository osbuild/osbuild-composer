package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/fsnode"
	"github.com/osbuild/images/internal/users"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type OSTreeRawImage struct {
	Base

	Platform       platform.Platform
	Workload       workload.Workload
	PartitionTable *disk.PartitionTable

	Users  []users.User
	Groups []users.Group

	CommitSource ostree.SourceSpec

	SysrootReadOnly bool

	Remote ostree.Remote
	OSName string

	KernelOptionsAppend []string
	Keyboard            string
	Locale              string

	Filename string

	Compression string

	Ignition bool

	Directories []*fsnode.Directory
	Files       []*fsnode.File
}

func NewOSTreeRawImage(commit ostree.SourceSpec) *OSTreeRawImage {
	return &OSTreeRawImage{
		Base:         NewBase("ostree-raw-image"),
		CommitSource: commit,
	}
}

func ostreeCompressedImagePipelines(img *OSTreeRawImage, m *manifest.Manifest, buildPipeline *manifest.Build) *manifest.XZ {
	imagePipeline := baseRawOstreeImage(img, m, buildPipeline)

	xzPipeline := manifest.NewXZ(m, buildPipeline, imagePipeline)
	xzPipeline.Filename = img.Filename

	return xzPipeline
}

func baseRawOstreeImage(img *OSTreeRawImage, m *manifest.Manifest, buildPipeline *manifest.Build) *manifest.RawOSTreeImage {
	osPipeline := manifest.NewOSTreeDeployment(m, buildPipeline, img.CommitSource, img.OSName, img.Ignition, img.Platform)
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

	return manifest.NewRawOStreeImage(m, buildPipeline, img.Platform, osPipeline)
}

func (img *OSTreeRawImage) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos)
	buildPipeline.Checkpoint()

	var art *artifact.Artifact
	switch img.Platform.GetImageFormat() {
	case platform.FORMAT_VMDK:
		if img.Compression != "" {
			panic(fmt.Sprintf("no compression is allowed with VMDK format for %q", img.name))
		}
		ostreeBase := baseRawOstreeImage(img, m, buildPipeline)
		vmdkPipeline := manifest.NewVMDK(m, buildPipeline, nil, ostreeBase)
		vmdkPipeline.Filename = img.Filename
		art = vmdkPipeline.Export()
	default:
		switch img.Compression {
		case "xz":
			ostreeCompressed := ostreeCompressedImagePipelines(img, m, buildPipeline)
			art = ostreeCompressed.Export()
		case "":
			ostreeBase := baseRawOstreeImage(img, m, buildPipeline)
			ostreeBase.Filename = img.Filename
			art = ostreeBase.Export()
		default:
			panic(fmt.Sprintf("unsupported compression type %q on %q", img.Compression, img.name))
		}
	}

	return art, nil
}
