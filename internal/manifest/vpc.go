package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild"
)

// A VPC turns a raw image file into qemu-based image format, such as qcow2.
type VPC struct {
	Base
	Filename string

	imgPipeline *RawImage
}

// NewVPC createsa new Qemu pipeline. imgPipeline is the pipeline producing the
// raw image. The pipeline name is the name of the new pipeline. Filename is the name
// of the produced image.
func NewVPC(m *Manifest,
	buildPipeline *Build,
	imgPipeline *RawImage) *VPC {
	p := &VPC{
		Base:        NewBase(m, "vpc", buildPipeline),
		imgPipeline: imgPipeline,
		Filename:    "image.vpc",
	}
	if imgPipeline.Base.manifest != m {
		panic("live image pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *VPC) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewQEMUStage(
		osbuild.NewQEMUStageOptions(p.Filename, osbuild.QEMUFormatVPC, nil),
		osbuild.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.Filename),
	))

	return pipeline
}

func (p *VPC) getBuildPackages() []string {
	return []string{"qemu-img"}
}
