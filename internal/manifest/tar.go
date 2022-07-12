package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild"
)

// A Tar represents the contents of another pipeline in a tar file
type Tar struct {
	Base
	Filename string

	inputPipeline *Base
}

// NewTar creates a new TarPipeline. The inputPipeline represents the
// filesystem tree which will be the contents of the tar file. The pipelinename
// is the name of the pipeline. The filename is the name of the output tar file.
func NewTar(m *Manifest,
	buildPipeline *Build,
	inputPipeline *Base,
	pipelinename string) *Tar {
	p := &Tar{
		Base:          NewBase(m, pipelinename, buildPipeline),
		inputPipeline: inputPipeline,
		Filename:      "image.tar",
	}
	if inputPipeline.manifest != m {
		panic("tree pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *Tar) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	tree := new(osbuild.TarStageInput)
	tree.Type = "org.osbuild.tree"
	tree.Origin = "org.osbuild.pipeline"
	tree.References = []string{"name:" + p.inputPipeline.Name()}
	tarStage := osbuild.NewTarStage(&osbuild.TarStageOptions{Filename: p.Filename}, &osbuild.TarStageInputs{Tree: tree})

	pipeline.AddStage(tarStage)

	return pipeline
}

func (p *Tar) getBuildPackages() []string {
	return []string{"tar"}
}
