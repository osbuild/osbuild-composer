package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// A VPC turns a raw image file into qemu-based image format, such as qcow2.
type VPC struct {
	Base

	imgPipeline *RawImage
	filename    string
}

// NewVPC createsa new Qemu pipeline. imgPipeline is the pipeline producing the
// raw image. The pipeline name is the name of the new pipeline. Filename is the name
// of the produced image.
func NewVPC(m *Manifest,
	buildPipeline *Build,
	imgPipeline *RawImage,
	filename string) *VPC {
	p := &VPC{
		Base:        NewBase(m, "vpc", buildPipeline),
		imgPipeline: imgPipeline,
		filename:    filename,
	}
	if imgPipeline.Base.manifest != m {
		panic("live image pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *VPC) serialize() osbuild2.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild2.NewQEMUStage(
		osbuild2.NewQEMUStageOptions(p.filename, osbuild2.QEMUFormatVPC, nil),
		osbuild2.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.filename),
	))

	return pipeline
}

func (p *VPC) getBuildPackages() []string {
	return []string{"qemu-img"}
}
