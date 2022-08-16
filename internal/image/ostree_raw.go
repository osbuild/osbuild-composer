package image

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

type OSTreeRawImage struct {
	Base
	Platform       platform.Platform
	Workload       workload.Workload
	PartitionTable *disk.PartitionTable

	OSTreeURL    string
	OSTreeRef    string
	OSTreeCommit string

	Remote string
	OSName string

	Filename string
}

func NewOSTreeRawImage() *OSTreeRawImage {
	return &OSTreeRawImage{
		Base: NewBase("ostree-raw-image"),
	}
}

func (img *OSTreeRawImage) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOSTreeDeployment(m, buildPipeline, img.OSTreeRef, img.OSTreeCommit, img.OSTreeURL, img.OSName, img.Remote, img.Platform)
	osPipeline.PartitionTable = img.PartitionTable

	imagePipeline := manifest.NewRawOStreeImage(m, buildPipeline, osPipeline)

	xzPipeline := manifest.NewXZ(m, buildPipeline, imagePipeline)
	xzPipeline.Filename = img.Filename

	art := xzPipeline.Export()

	return art, nil
}
