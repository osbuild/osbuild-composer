package image

import (
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/experimentalflags"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/runner"
)

// addBuildBootstrapPipelines will add a build pipeline to the
// manifest. It will also conditionally add a bootstrap stage
// only if the manifest contains a Bootstrap config or the
// "IMAGE_BUILDER_EXPERIMENTAL=bootstrap=<container-ref>" env
// is set.
//
// Two bootstrap strategies are supported:
//   - container: deploys a container image as the bootstrap root
//   - packages: installs RPMs with ignorearch to create the
//     bootstrap root (no pre-built container needed)
func addBuildBootstrapPipelines(m *manifest.Manifest, runner runner.Runner, repos []rpmmd.RepoConfig, opts *manifest.BuildOptions) manifest.Build {
	if opts == nil {
		opts = &manifest.BuildOptions{}
	}

	// check for experimental override first
	if overrideRef := experimentalflags.String("bootstrap"); overrideRef != "" {
		m.Bootstrap = &manifest.BootstrapConfig{ContainerRef: overrideRef}
	}

	// no bootstrap wanted
	if m.Bootstrap == nil {
		return manifest.NewBuild(m, runner, repos, opts)
	}

	if len(m.Bootstrap.Packages) > 0 {
		bootstrapPipeline := manifest.NewBootstrapFromPackages(m, repos, m.Bootstrap.Packages)
		opts.BootstrapPipeline = bootstrapPipeline
		opts.DisableSELinux = true
		return manifest.NewBuild(m, runner, repos, opts)
	}

	cntSrcs := []container.SourceSpec{
		{
			Source: m.Bootstrap.ContainerRef,
			Name:   m.Bootstrap.ContainerRef,
		},
	}
	bootstrapPipeline := manifest.NewBootstrap(m, cntSrcs)
	opts.BootstrapPipeline = bootstrapPipeline
	opts.DisableSELinux = true
	return manifest.NewBuild(m, runner, repos, opts)
}
