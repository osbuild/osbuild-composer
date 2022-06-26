package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type BuildPipeline struct {
	Pipeline

	repos        []rpmmd.RepoConfig
	packageSpecs []rpmmd.PackageSpec
}

func NewBuildPipeline(runner string, repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec) BuildPipeline {
	pipeline := BuildPipeline{
		Pipeline:     New("build", nil, &runner),
		repos:        repos,
		packageSpecs: packages,
	}
	return pipeline
}

func (p BuildPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	pipeline.AddStage(osbuild2.NewRPMStage(osbuild2.NewRPMStageOptions(p.repos), osbuild2.NewRpmStageSourceFilesInputs(p.packageSpecs)))
	pipeline.AddStage(osbuild2.NewSELinuxStage(selinuxStageOptions(true)))

	return pipeline
}
