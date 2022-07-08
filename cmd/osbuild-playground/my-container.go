package main

import (
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
func (img *MyContainer) InstantiateManifest(m *manifest.Manifest, repos []rpmmd.RepoConfig, runner runner.Runner) error {
	// Let's create a simple OCI container!

	// configure a build pipeline
	build := manifest.NewBuild(m, runner, repos)

	// create a minimal non-bootable OS tree
	os := manifest.NewOS(m, build, &platform.X86{}, repos)

	// create an OCI container containing the OS tree created above
	manifest.NewOCIContainer(m, build, os)

	return nil
}

// GetExports returns a list of the pipelines osbuild should export.
// These are the pipelines containing the artefact you want returned.
//
// TODO: Move this to be implemented in terms ofthe Manifest package.
//       We should not need to know the pipeline names.
func (img *MyContainer) GetExports() []string {
	return []string{"container"}
}

// GetCheckpoints returns a list of the pipelines osbuild should
// checkpoint. These are the pipelines likely to be reusable in
// future runs.
//
// TODO: Move this to be implemented in terms ofthe Manifest package.
//       We should not need to know the pipeline names.
func (img *MyContainer) GetCheckpoints() []string {
	return []string{"build"}
}
