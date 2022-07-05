package main

import (
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// MyOptions contains the arguments passed in as a JSON blob.
// You can replace them with whatever you want to use to
// configure your image. In the current example they are
// unused.
type MyOptions struct {
	MyOption string `json:"my_option"`
}

// Build your manifest by attaching pipelines to it
//
// @m is the manifest you are constructing
// @options are what was passed in on the commandline
// @repos are the default repositories for the host OS/arch
// @runner is needed by any build pipelines
//
// Return nil when you are done, or an error if something
// went wrong. Your manifest will be streamed to stdout and
// can be piped directly to either jq for inspection or
// osbuild for building.
func MyManifest(m *manifest.Manifest, options *MyOptions, repos []rpmmd.RepoConfig, runner string) error {
	// Let's create a simple OCI container!

	// configure a build pipeline
	build := manifest.NewBuildPipeline(m, runner, repos)

	// create a non-bootable OS tree containing the `core` comps group
	os := manifest.NewOSPipeline(m, build, &platform.X86{}, repos)
	os.ExtraBasePackages = []string{
		"@core",
	}

	// create an OCI container containing the OS tree created above
	manifest.NewOCIContainerPipeline(m, build, &os.BasePipeline, "x86_64", "my-container.tar")

	return nil
}
