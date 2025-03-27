package image

import (
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/experimentalflags"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

// addBuildBootstrapPipelines will add a build pipeline to the
// manifest. It will also conditionally add a container bootstrap
// stage only if the manifest contains DistroBootstrapRef or the
// "IMAGE_BUILDER_EXPERIMENTAL=bootstrap=<container-ref>" env
// is set.
//
// This bootstrap stage allows us to do cross-arch/cross-distro builds
// by bootstraping the buildroot rpm installs with the bootstap container
// as the "bootstrap-buildroot".
//
// A "bootstrap" container has only these requirements:
//   - python3 for the runners
//   - mount for some of the bind mounting that osbuild does (if we would
//     move all mounts to internal code this requirement could go away)
//   - rpm so that the real buildroot rpms can get installed
//
// (and does not even need a working dnf or repo setup).
func addBuildBootstrapPipelines(m *manifest.Manifest, runner runner.Runner, repos []rpmmd.RepoConfig, opts *manifest.BuildOptions) manifest.Build {
	bootstrapBuildrootRef := experimentalflags.String("bootstrap")
	if bootstrapBuildrootRef == "" {
		bootstrapBuildrootRef = m.DistroBootstrapRef
	}
	// no bootstrap pipeline wanted, we can finish early
	if bootstrapBuildrootRef == "" {
		return manifest.NewBuild(m, runner, repos, opts)
	}

	// add the bootstrap pipeline
	cntSrcs := []container.SourceSpec{
		{
			Source: bootstrapBuildrootRef,
			Name:   bootstrapBuildrootRef,
		},
	}
	if opts == nil {
		opts = &manifest.BuildOptions{}
	}
	bootstrapPipeline := manifest.NewBootstrap(m, cntSrcs)
	opts.BootstrapPipeline = bootstrapPipeline
	// This is currently needed for bootstraped buildroot
	// containers because most of the boostrap containers do not
	// include setfiles(8) and when constructing the buildroot
	// setfiles(8) is called as the last step by default.
	opts.DisableSELinux = true
	return manifest.NewBuild(m, runner, repos, opts)
}
