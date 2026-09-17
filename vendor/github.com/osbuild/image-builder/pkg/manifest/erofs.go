package manifest

import (
	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

type Erofs struct {
	Base
	filename      string
	inputPipeline Pipeline
}

func NewErofs(buildPipeline Build, inputPipeline Pipeline, pipelinename string) *Erofs {
	p := &Erofs{
		Base:          NewBase(pipelinename, buildPipeline),
		inputPipeline: inputPipeline,
		filename:      "image.raw",
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *Erofs) Filename() string {
	return p.filename
}

func (p *Erofs) SetFilename(filename string) {
	p.filename = filename
}

func (p *Erofs) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	erofsOptions := osbuild.ErofsStageOptions{
		Filename: p.filename,
	}
	pipeline.AddStage(osbuild.NewErofsStage(erofsOptions, p.inputPipeline.Name()))

	return pipeline, nil
}

func (p *Erofs) getBuildPackages(Distro) ([]string, error) {
	return []string{"erofs-utils"}, nil
}

func (p *Erofs) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename(), nil)
}
