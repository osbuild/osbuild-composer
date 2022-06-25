package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type OCIContainerPipeline struct {
	Pipeline
	treePipeline *Pipeline
	Architecture string
	Filename     string
	Cmd          []string
	ExposedPorts []string
}

func NewOCIContainerPipeline(buildPipeline *BuildPipeline, treePipeline *Pipeline) OCIContainerPipeline {
	return OCIContainerPipeline{
		Pipeline:     New("container", &buildPipeline.Pipeline),
		treePipeline: treePipeline,
	}
}

func (p OCIContainerPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	options := &osbuild2.OCIArchiveStageOptions{
		Architecture: p.Architecture,
		Filename:     p.Filename,
		Config: &osbuild2.OCIArchiveConfig{
			Cmd:          p.Cmd,
			ExposedPorts: p.ExposedPorts,
		},
	}
	baseInput := new(osbuild2.OCIArchiveStageInput)
	baseInput.Type = "org.osbuild.tree"
	baseInput.Origin = "org.osbuild.pipeline"
	baseInput.References = []string{"name:" + p.treePipeline.Name()}
	inputs := &osbuild2.OCIArchiveStageInputs{Base: baseInput}
	pipeline.AddStage(osbuild2.NewOCIArchiveStage(options, inputs))

	return pipeline
}
