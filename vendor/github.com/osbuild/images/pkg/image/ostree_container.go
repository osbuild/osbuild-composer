package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type OSTreeContainer struct {
	Base
	OSCustomizations                 manifest.OSCustomizations
	OSTreeCommitServerCustomizations manifest.OSTreeCommitServerCustomizations
	OCIContainerCustomizations       manifest.OCIContainerCustomizations
	Environment                      environment.Environment

	// OSTreeParent specifies the source for an optional parent commit for the
	// new commit being built.
	OSTreeParent *ostree.SourceSpec

	// OSTreeRef is the ref of the commit that will be built.
	OSTreeRef string

	OSVersion              string
	ExtraContainerPackages rpmmd.PackageSet // FIXME: this is never read
	ContainerLanguage      string
}

func NewOSTreeContainer(platform platform.Platform, filename string, ref string) *OSTreeContainer {
	return &OSTreeContainer{
		Base: NewBase("ostree-container", platform, filename),

		OSTreeRef: ref,
	}
}

func (img *OSTreeContainer) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(buildPipeline, img.platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.OSTreeRef = img.OSTreeRef
	osPipeline.OSTreeParent = img.OSTreeParent

	commitPipeline := manifest.NewOSTreeCommit(buildPipeline, osPipeline, img.OSTreeRef)
	commitPipeline.OSVersion = img.OSVersion

	if img.OSTreeCommitServerCustomizations.OSTreeServer == nil {
		return nil, fmt.Errorf("missing ostree_server image config")
	}

	serverPipeline := manifest.NewOSTreeCommitServer(
		buildPipeline,
		img.platform,
		repos,
		commitPipeline,
	)
	serverPipeline.OSTreeCommitServerCustomizations = img.OSTreeCommitServerCustomizations
	serverPipeline.Language = img.ContainerLanguage

	containerPipeline := manifest.NewOCIContainer(buildPipeline, serverPipeline)
	containerPipeline.OCIContainerCustomizations = img.OCIContainerCustomizations

	containerPipeline.SetFilename(img.filename)
	artifact := containerPipeline.Export()

	return artifact, nil
}
