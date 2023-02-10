package image

import (
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

type OSTreeRawImage struct {
	Base

	Platform       platform.Platform
	Workload       workload.Workload
	PartitionTable *disk.PartitionTable

	Users  []users.User
	Groups []users.Group

	Commit ostree.CommitSpec

	SysrootReadOnly bool

	Remote ostree.Remote
	OSName string

	KernelOptionsAppend []string
	Keyboard            string
	Locale              string

	Filename string

	Ignition bool

	Directories []*fsnode.Directory
	Files       []*fsnode.File
}

func NewOSTreeRawImage(commit ostree.CommitSpec) *OSTreeRawImage {
	return &OSTreeRawImage{
		Base:   NewBase("ostree-raw-image"),
		Commit: commit,
	}
}

func ostreeCompressedImagePipelines(img *OSTreeRawImage, m *manifest.Manifest, buildPipeline *manifest.Build) *manifest.XZ {
	osPipeline := manifest.NewOSTreeDeployment(m, buildPipeline, img.Commit, img.OSName, img.Ignition, img.Platform)
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

	imagePipeline := manifest.NewRawOStreeImage(m, buildPipeline, img.Platform, osPipeline)

	xzPipeline := manifest.NewXZ(m, buildPipeline, imagePipeline)
	xzPipeline.Filename = img.Filename

	return xzPipeline
}

func (img *OSTreeRawImage) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos)
	buildPipeline.Checkpoint()

	xzPipeline := ostreeCompressedImagePipelines(img, m, buildPipeline)

	art := xzPipeline.Export()
	return art, nil
}
