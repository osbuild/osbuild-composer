package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// A BuildPipeline represents the build environment for other pipelines. As a
// general rule, tools required to build pipelines are used from the build
// environment, rather than from the pipeline itself. Without a specified
// build environment, the build host's root filesystem would be used, which
// is not predictable nor reproducible. For the purposes of building the
// build pipeline, we do use the build host's filesystem, this means we should
// make minimal assumptions about what's available there.
type BuildPipeline struct {
	BasePipeline

	repos        []rpmmd.RepoConfig
	packageSpecs []rpmmd.PackageSpec
}

// NewBuildPipeline creates a new build pipeline from the repositories in repos
// and the specified packages.
func NewBuildPipeline(runner string, repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec) BuildPipeline {
	pipeline := BuildPipeline{
		BasePipeline: NewBasePipeline("build", nil, &runner),
		repos:        repos,
		packageSpecs: packages,
	}
	return pipeline
}

func (p BuildPipeline) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p BuildPipeline) serialize() osbuild2.Pipeline {
	pipeline := p.BasePipeline.serialize()

	pipeline.AddStage(osbuild2.NewRPMStage(osbuild2.NewRPMStageOptions(p.repos), osbuild2.NewRpmStageSourceFilesInputs(p.packageSpecs)))
	pipeline.AddStage(osbuild2.NewSELinuxStage(&osbuild2.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
		Labels: map[string]string{
			"/usr/bin/cp": "system_u:object_r:install_exec_t:s0",
		},
	},
	))

	return pipeline
}
