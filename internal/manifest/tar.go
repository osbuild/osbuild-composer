package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// A TarPipeline represents the contents of another pipeline in a tar file
type TarPipeline struct {
	BasePipeline
	inputPipeline *BasePipeline
	filename      string
}

// NewTarPipeline creates a new TarPipeline. The inputPipeline represents the
// filesystem tree which will be the contents of the tar file. The pipelinename
// is the name of the pipeline. The filename is the name of the output tar file.
func NewTarPipeline(m *Manifest,
	buildPipeline *BuildPipeline,
	inputPipeline *BasePipeline,
	pipelinename,
	filename string) *TarPipeline {
	p := &TarPipeline{
		BasePipeline:  NewBasePipeline(m, pipelinename, buildPipeline, nil),
		inputPipeline: inputPipeline,
		filename:      filename,
	}
	if inputPipeline.manifest != m {
		panic("tree pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *TarPipeline) serialize() osbuild2.Pipeline {
	pipeline := p.BasePipeline.serialize()

	tree := new(osbuild2.TarStageInput)
	tree.Type = "org.osbuild.tree"
	tree.Origin = "org.osbuild.pipeline"
	tree.References = []string{"name:" + p.inputPipeline.Name()}
	tarStage := osbuild2.NewTarStage(&osbuild2.TarStageOptions{Filename: p.filename}, &osbuild2.TarStageInputs{Tree: tree})

	pipeline.AddStage(tarStage)

	return pipeline
}
