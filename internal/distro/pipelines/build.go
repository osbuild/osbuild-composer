package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type BuildPipeline struct {
	Pipeline
	Repos        []rpmmd.RepoConfig
	PackageSpecs []rpmmd.PackageSpec
}

func NewBuildPipeline(runner string) BuildPipeline {
	pipeline := BuildPipeline{
		Pipeline: New("build", nil),
	}
	pipeline.runner = &runner
	return pipeline
}

func (p BuildPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	pipeline.AddStage(osbuild2.NewRPMStage(osbuild2.NewRPMStageOptions(p.Repos), osbuild2.NewRpmStageSourceFilesInputs(p.PackageSpecs)))
	pipeline.AddStage(osbuild2.NewSELinuxStage(selinuxStageOptions(true)))

	return pipeline
}
