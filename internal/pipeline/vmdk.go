package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// A VMDKPipeline turns a raw image file into vmdk image.
type VMDKPipeline struct {
	Pipeline

	imgPipeline *LiveImgPipeline
	filename    string
}

// NewVMDKPipeline creates a new VMDK pipeline. imgPipeline is the pipeline producing the
// raw image. Filename is the name of the produced image.
func NewVMDKPipeline(buildPipeline *BuildPipeline, imgPipeline *LiveImgPipeline, filename string) VMDKPipeline {
	return VMDKPipeline{
		Pipeline:    New("vmdk", buildPipeline, nil),
		imgPipeline: imgPipeline,
		filename:    filename,
	}
}

func (p VMDKPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	pipeline.AddStage(osbuild2.NewQEMUStage(
		osbuild2.NewQEMUStageOptions(p.filename, osbuild2.QEMUFormatVMDK, osbuild2.VMDKOptions{
			Subformat: osbuild2.VMDKSubformatStreamOptimized,
		}),
		osbuild2.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.Filename()),
	))

	return pipeline
}
