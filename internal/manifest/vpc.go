package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// A VPCPipeline turns a raw image file into qemu-based image format, such as qcow2.
type VPCPipeline struct {
	BasePipeline

	imgPipeline *LiveImgPipeline
	filename    string
}

// NewVPCPipeline createsa new Qemu pipeline. imgPipeline is the pipeline producing the
// raw image. The pipeline name is the name of the new pipeline. Filename is the name
// of the produced image.
func NewVPCPipeline(m *Manifest,
	buildPipeline *BuildPipeline,
	imgPipeline *LiveImgPipeline,
	filename string) *VPCPipeline {
	p := &VPCPipeline{
		BasePipeline: NewBasePipeline(m, "vpc", buildPipeline, nil),
		imgPipeline:  imgPipeline,
		filename:     filename,
	}
	if imgPipeline.BasePipeline.manifest != m {
		panic("live image pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *VPCPipeline) serialize() osbuild2.Pipeline {
	pipeline := p.BasePipeline.serialize()

	pipeline.AddStage(osbuild2.NewQEMUStage(
		osbuild2.NewQEMUStageOptions(p.filename, osbuild2.QEMUFormatVPC, nil),
		osbuild2.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.filename),
	))

	return pipeline
}
