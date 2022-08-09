package main

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
)

// MyContainer contains the arguments passed in as a JSON blob.
// You can replace them with whatever you want to use to
// configure your image. In the current example they are
// unused.
type MyContainer struct {
	MyOption string `json:"my_option"`
}

// Name returns the name of the image type, used to select what kind
// of image to build.
func (img *MyContainer) Name() string {
	return "my-container"
}

// init registeres this image type
func init() {
	AddImageType(&MyContainer{})
}

// Build your manifest by attaching pipelines to it
//
// @m is the manifest you are constructing
// @options are what was passed in on the commandline
// @repos are the default repositories for the host OS/arch
// @runner is needed by any build pipelines
//
// Return nil when you are done, or an error if something
// went wrong. Your manifest will be streamed to osbuild
// for building.
func (img *MyContainer) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	// Let's create a simple OCI container!

	// configure a build pipeline
	build := manifest.NewBuild(m, runner, repos)
	build.Checkpoint()

	// create a minimal non-bootable OS tree
	os := manifest.NewOS(m, build, &platform.X86{}, repos)
	os.ExtraBasePackages = []string{"@core"}
	os.OSCustomizations.Language = "en_US.UTF-8"
	os.OSCustomizations.Hostname = "my-host"
	os.OSCustomizations.Timezone = "UTC"

	// create an OCI container containing the OS tree created above
	container := manifest.NewOCIContainer(m, build, os)
	artifact := container.Export()

	return artifact, nil
}
