package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// A QeumPipeline turns a raw image file into qemu-based image format, such as qcow2.
type QemuPipeline struct {
	Pipeline
	// TODO: don't expose the osbuild2 types in the API
	Format        osbuild2.QEMUFormat
	FormatOptions osbuild2.QEMUFormatOptions

	imgPipeline *LiveImgPipeline
	filename    string
}

// NewQemuPipeline createsa new Qemu pipeline. imgPipeline is the pipeline producing the
// raw image. The pipeline name is the name of the new pipeline. Filename is the name
// of the produced image.
func NewQemuPipeline(buildPipeline *BuildPipeline, imgPipeline *LiveImgPipeline, pipelinename, filename string) QemuPipeline {
	return QemuPipeline{
		Pipeline:    New(pipelinename, buildPipeline, nil),
		imgPipeline: imgPipeline,
		filename:    filename,
	}
}

func (p QemuPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	pipeline.AddStage(osbuild2.NewQEMUStage(
		osbuild2.NewQEMUStageOptions(p.filename, p.Format, p.FormatOptions),
		osbuild2.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.Filename()),
	))

	return pipeline
}
