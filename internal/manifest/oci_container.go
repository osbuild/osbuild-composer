package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// An OCIContainerPipeline represents an OCI container, containing a filesystem
// tree created by another Pipeline.
type OCIContainerPipeline struct {
	BasePipeline
	Cmd          []string
	ExposedPorts []string

	treePipeline *BasePipeline
	architecture string
	filename     string
}

func NewOCIContainerPipeline(buildPipeline *BuildPipeline, treePipeline *BasePipeline, architecture, filename string) OCIContainerPipeline {
	return OCIContainerPipeline{
		BasePipeline: NewBasePipeline("container", buildPipeline, nil),
		treePipeline: treePipeline,
		architecture: architecture,
		filename:     filename,
	}
}

func (p OCIContainerPipeline) serialize() osbuild2.Pipeline {
	pipeline := p.BasePipeline.serialize()

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
