package image

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/environment"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

type OSTreeArchive struct {
	Base
	Platform         platform.Platform
	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment
	Workload         workload.Workload
	OSTreeParent     manifest.OSTree
	OSTreeRef        string
	OSVersion        string
	Filename         string
}

func NewOSTreeArchive() *OSTreeArchive {
	return &OSTreeArchive{
		Base: NewBase("ostree-archive"),
	}
}

func (img *OSTreeArchive) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(m, buildPipeline, img.Platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.Workload = img.Workload
	osPipeline.OSTree = &img.OSTreeParent

	ostreeCommitPipeline := manifest.NewOSTreeCommit(m, buildPipeline, osPipeline, img.OSTreeRef)
	ostreeCommitPipeline.OSVersion = img.OSVersion

	tarPipeline := manifest.NewTar(m, buildPipeline, &ostreeCommitPipeline.Base, "commit-archive")
	tarPipeline.Filename = img.Filename
	artifact := tarPipeline.Export()

	return artifact, nil
}
