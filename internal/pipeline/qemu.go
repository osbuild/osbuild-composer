package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type QemuPipeline struct {
	Pipeline
	imgPipeline    *Pipeline
	InputFilename  string
	OutputFilename string
	// TODO: don't expose the osbuild2 types in the API
	Format        osbuild2.QEMUFormat
	FormatOptions osbuild2.QEMUFormatOptions
}

func NewQemuPipeline(buildPipeline *BuildPipeline, imgPipeline *LiveImgPipeline, name string) QemuPipeline {
	return QemuPipeline{
		Pipeline:    New(name, buildPipeline, nil),
		imgPipeline: &imgPipeline.Pipeline,
	}
}

func (p QemuPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	pipeline.AddStage(osbuild2.NewQEMUStage(
		osbuild2.NewQEMUStageOptions(p.OutputFilename, p.Format, p.FormatOptions),
		osbuild2.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.InputFilename),
	))

	return pipeline
}
