package image

import (
	"math/rand"

	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/customizations/bootc"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type OSTreeArchive struct {
	Base
	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment

	// OSTreeParent specifies the source for an optional parent commit for the
	// new commit being built.
	OSTreeParent *ostree.SourceSpec

	// OSTreeRef is the ref of the commit that will be built.
	OSTreeRef string

	OSVersion string

	InstallWeakDeps bool

	BootContainer bool

	// bootc install config for defining the preferred filesystem type and
	// kernel arguments for bootable containers.
	// This is ignored if BootContainer = false.
	BootcConfig *bootc.Config

	// Bootupd enables bootupd metadata generation for ostree commits.
	// When true, runs bootupctl backend generate-update-metadata to
	// transform /usr/lib/ostree-boot into bootupd-compatible update metadata.
	Bootupd bool
}

func NewOSTreeArchive(platform platform.Platform, filename string, ref string) *OSTreeArchive {
	return &OSTreeArchive{
		Base:            NewBase("ostree-archive", platform, filename),
		OSTreeRef:       ref,
		InstallWeakDeps: true,
	}
}

func (img *OSTreeArchive) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(buildPipeline, img.platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.OSTreeParent = img.OSTreeParent
	osPipeline.OSTreeRef = img.OSTreeRef
	osPipeline.OSCustomizations.InstallWeakDeps = img.InstallWeakDeps

	ostreeCommitPipeline := manifest.NewOSTreeCommit(buildPipeline, osPipeline, img.OSTreeRef)
	ostreeCommitPipeline.OSVersion = img.OSVersion

	// Enable bootupd metadata generation if requested
	if img.Bootupd {
		osPipeline.Bootupd = true
	}

	var artifact *artifact.Artifact
	if img.BootContainer {
		osPipeline.Bootupd = true
		osPipeline.BootcConfig = img.BootcConfig
		encapsulatePipeline := manifest.NewOSTreeEncapsulate(buildPipeline, ostreeCommitPipeline, "ostree-encapsulate")
		encapsulatePipeline.SetFilename(img.filename)
		artifact = encapsulatePipeline.Export()
	} else {
		tarPipeline := manifest.NewTar(buildPipeline, ostreeCommitPipeline, "commit-archive")
		tarPipeline.SetFilename(img.filename)
		artifact = tarPipeline.Export()
	}

	return artifact, nil
}
