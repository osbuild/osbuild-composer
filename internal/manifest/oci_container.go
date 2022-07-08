package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// An OCIContainer represents an OCI container, containing a filesystem
// tree created by another Pipeline.
type OCIContainer struct {
	Base
	Cmd          []string
	ExposedPorts []string

	treePipeline *Base
	architecture string
	filename     string
}

func NewOCIContainer(m *Manifest,
	buildPipeline *Build,
	treePipeline *Base,
	architecture,
	filename string) *OCIContainer {
	p := &OCIContainer{
		Base:         NewBase(m, "container", buildPipeline),
		treePipeline: treePipeline,
		architecture: architecture,
		filename:     filename,
	}
	if treePipeline.build.Base.manifest != m {
		panic("tree pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *OCIContainer) serialize() osbuild2.Pipeline {
	pipeline := p.Base.serialize()

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

func (p *OCIContainer) getBuildPackages() []string {
	return []string{"tar"}
}
