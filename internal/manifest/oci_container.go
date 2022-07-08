package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// An OCIContainer represents an OCI container, containing a filesystem
// tree created by another Pipeline.
type OCIContainer struct {
	Base
	Filename     string
	Cmd          []string
	ExposedPorts []string

	treePipeline Tree
}

func NewOCIContainer(m *Manifest,
	buildPipeline *Build,
	treePipeline Tree) *OCIContainer {
	p := &OCIContainer{
		Base:         NewBase(m, "container", buildPipeline),
		treePipeline: treePipeline,
		Filename:     "oci-archive.tar",
	}
	if treePipeline.GetManifest() != m {
		panic("tree pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *OCIContainer) serialize() osbuild2.Pipeline {
	pipeline := p.Base.serialize()

	options := &osbuild2.OCIArchiveStageOptions{
		Architecture: p.treePipeline.GetPlatform().GetArch().String(),
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

func (p *OCIContainer) getBuildPackages() []string {
	return []string{"tar"}
}
