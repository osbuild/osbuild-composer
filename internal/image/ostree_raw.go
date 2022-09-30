package image

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/disk"
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

	Remote ostree.Remote
	OSName string

	KernelOptionsAppend []string
	Keyboard            string
	Locale              string

	Filename string
}

func NewOSTreeRawImage(commit ostree.CommitSpec) *OSTreeRawImage {
	return &OSTreeRawImage{
		Base:   NewBase("ostree-raw-image"),
		Commit: commit,
	}
}

func (img *OSTreeRawImage) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOSTreeDeployment(m, buildPipeline, img.Commit, img.OSName, img.Platform)
	osPipeline.PartitionTable = img.PartitionTable
	osPipeline.Remote = img.Remote
	osPipeline.KernelOptionsAppend = img.KernelOptionsAppend
	osPipeline.Keyboard = img.Keyboard
	osPipeline.Locale = img.Locale
	osPipeline.Users = img.Users
	osPipeline.Groups = img.Groups

	imagePipeline := manifest.NewRawOStreeImage(m, buildPipeline, img.Platform, osPipeline)

	xzPipeline := manifest.NewXZ(m, buildPipeline, imagePipeline)
	xzPipeline.Filename = img.Filename

	art := xzPipeline.Export()

	return art, nil
}
