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

	dependents   []Pipeline
	repos        []rpmmd.RepoConfig
	packageSpecs []rpmmd.PackageSpec
}

// NewBuildPipeline creates a new build pipeline from the repositories in repos
// and the specified packages.
func NewBuildPipeline(m *Manifest, runner string, repos []rpmmd.RepoConfig) *BuildPipeline {
	pipeline := &BuildPipeline{
		BasePipeline: NewBasePipeline(m, "build", nil, &runner),
		dependents:   make([]Pipeline, 0),
		repos:        repos,
	}
	m.addPipeline(pipeline)
	return pipeline
}

func (p *BuildPipeline) addDependent(dep Pipeline) {
	p.dependents = append(p.dependents, dep)
}

func (p *BuildPipeline) getPackageSetChain() []rpmmd.PackageSet {
	// TODO: break apart into individual pipelines
	packages := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"policycoreutils",
		"qemu-img",
		"selinux-policy-targeted",
		"systemd",
		"tar",
		"xz",
	}

	for _, pipeline := range p.dependents {
		packages = append(packages, pipeline.getBuildPackages()...)
	}

	return []rpmmd.PackageSet{
		{
			Include:      packages,
			Repositories: p.repos,
		},
	}
}

func (p *BuildPipeline) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p *BuildPipeline) serializeStart(packages []rpmmd.PackageSpec) {
	if len(p.packageSpecs) > 0 {
		panic("double call to serializeStart()")
	}
	p.packageSpecs = packages
}

func (p *BuildPipeline) serializeEnd() {
	if len(p.packageSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.packageSpecs = nil
}

func (p *BuildPipeline) serialize() osbuild2.Pipeline {
	if len(p.packageSpecs) == 0 {
		panic("serialization not started")
	}
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
