package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// An OCIContainerPipeline represents an OCI container, containing a filesystem
// tree created by another Pipeline.
type OCIContainerPipeline struct {
	Pipeline
	Cmd          []string
	ExposedPorts []string

	treePipeline *Pipeline
	architecture string
	filename     string
}

func NewOCIContainerPipeline(buildPipeline *BuildPipeline, treePipeline *Pipeline, architecture, filename string) OCIContainerPipeline {
	return OCIContainerPipeline{
		Pipeline:     New("container", buildPipeline, nil),
		treePipeline: treePipeline,
		architecture: architecture,
		filename:     filename,
	}
}

func (p OCIContainerPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	options := &osbuild2.OCIArchiveStageOptions{
		Architecture: p.architecture,
		Filename:     p.filename,
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
