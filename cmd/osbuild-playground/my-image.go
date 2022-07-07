package main

import (
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func init() {
	AddImageType(&MyImage{})
}

// MyImage contains the arguments passed in as a JSON blob.
// You can replace them with whatever you want to use to
// configure your image. In the current example they are
// unused.
type MyImage struct {
	MyOption string `json:"my_option"`
	Filename string `json:"filename"`
}

// Name returns the name of the image type, used to select what kind
// of image to build.
func (img *MyImage) Name() string {
	return "my-image"
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
func (img *MyImage) InstantiateManifest(m *manifest.Manifest, repos []rpmmd.RepoConfig, runner string) error {
	// Let's create a simple OCI container!

	// configure a build pipeline
	build := manifest.NewBuildPipeline(m, runner, repos)

	// create a non-bootable OS tree containing the `core` comps group
	os := manifest.NewOSPipeline(m, build, &platform.X86{}, repos)
	os.ExtraBasePackages = []string{
		"@core",
	}

	filename := "my-container.tar"
	if img.Filename != "" {
		filename = img.Filename
	}
	// create an OCI container containing the OS tree created above
	manifest.NewOCIContainerPipeline(m, build, &os.BasePipeline, "x86_64", filename)

	return nil
}

// GetExports returns a list of the pipelines osbuild should export.
// These are the pipelines containing the artefact you want returned.
//
// TODO: Move this to be implemented in terms ofthe Manifest package.
//       We should not need to know the pipeline names.
func (img *MyImage) GetExports() []string {
	return []string{"container"}
}

// GetCheckpoints returns a list of the pipelines osbuild should
// checkpoint. These are the pipelines likely to be reusable in
// future runs.
//
// TODO: Move this to be implemented in terms ofthe Manifest package.
//       We should not need to know the pipeline names.
func (img *MyImage) GetCheckpoints() []string {
	return []string{"build"}
}
