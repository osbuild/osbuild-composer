package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// A VMDKPipeline turns a raw image file into vmdk image.
type VMDKPipeline struct {
	BasePipeline

	imgPipeline *LiveImgPipeline
	filename    string
}

// NewVMDKPipeline creates a new VMDK pipeline. imgPipeline is the pipeline producing the
// raw image. Filename is the name of the produced image.
func NewVMDKPipeline(m *Manifest,
	buildPipeline *BuildPipeline,
	imgPipeline *LiveImgPipeline,
	filename string) *VMDKPipeline {
	p := &VMDKPipeline{
		BasePipeline: NewBasePipeline(m, "vmdk", buildPipeline, nil),
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

func (p *VMDKPipeline) serialize() osbuild2.Pipeline {
	pipeline := p.BasePipeline.serialize()

	pipeline.AddStage(osbuild2.NewQEMUStage(
		osbuild2.NewQEMUStageOptions(p.filename, osbuild2.QEMUFormatVMDK, osbuild2.VMDKOptions{
			Subformat: osbuild2.VMDKSubformatStreamOptimized,
		}),
		osbuild2.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.filename),
	))

	return pipeline
}
