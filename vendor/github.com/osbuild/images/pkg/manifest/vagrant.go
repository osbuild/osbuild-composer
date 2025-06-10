package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

type Vagrant struct {
	Base
	filename string

	imgPipeline FilePipeline
}

func (p Vagrant) Filename() string {
	return p.filename
}

func (p *Vagrant) SetFilename(filename string) {
	p.filename = filename
}

func NewVagrant(buildPipeline Build, imgPipeline FilePipeline) *Vagrant {
	p := &Vagrant{
		Base:        NewBase("vagrant", buildPipeline),
		imgPipeline: imgPipeline,
		filename:    "image.box",
	}

	if buildPipeline != nil {
		buildPipeline.addDependent(p)
	} else {
		imgPipeline.Manifest().addPipeline(p)
	}

	return p
}

func (p *Vagrant) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewVagrantStage(
		osbuild.NewVagrantStageOptions(osbuild.VagrantProviderLibvirt),
		osbuild.NewVagrantStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.Filename()),
	))

	return pipeline
}

func (p *Vagrant) getBuildPackages(Distro) []string {
	return []string{"qemu-img"}
}

func (p *Vagrant) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-qemu-disk"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
