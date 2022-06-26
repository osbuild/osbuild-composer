package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// A VPCPipeline turns a raw image file into qemu-based image format, such as qcow2.
type VPCPipeline struct {
	Pipeline

	imgPipeline *LiveImgPipeline
	filename    string
}

// NewVPCPipeline createsa new Qemu pipeline. imgPipeline is the pipeline producing the
// raw image. The pipeline name is the name of the new pipeline. Filename is the name
// of the produced image.
func NewVPCPipeline(buildPipeline *BuildPipeline, imgPipeline *LiveImgPipeline, filename string) VPCPipeline {
	return VPCPipeline{
		Pipeline:    New("vpc", buildPipeline, nil),
		imgPipeline: imgPipeline,
		filename:    filename,
	}
}

func (p VPCPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	pipeline.AddStage(osbuild2.NewQEMUStage(
		osbuild2.NewQEMUStageOptions(p.filename, osbuild2.QEMUFormatVPC, nil),
		osbuild2.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.Filename()),
	))

	return pipeline
}
