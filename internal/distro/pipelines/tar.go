package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type TarPipeline struct {
	Pipeline
	inputPipeline *Pipeline
	Filename      string
}

func NewTarPipeline(buildPipeline *BuildPipeline, inputPipeline *Pipeline, name string) TarPipeline {
	return TarPipeline{
		Pipeline:      New(name, &buildPipeline.Pipeline),
		inputPipeline: inputPipeline,
	}
}

func (p TarPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	tree := new(osbuild2.TarStageInput)
	tree.Type = "org.osbuild.tree"
	tree.Origin = "org.osbuild.pipeline"
	tree.References = []string{"name:" + p.inputPipeline.Name()}
	tarStage := osbuild2.NewTarStage(&osbuild2.TarStageOptions{Filename: p.Filename}, &osbuild2.TarStageInputs{Tree: tree})

	pipeline.AddStage(tarStage)

	return pipeline
}
