package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// A QCOW2Pipeline turns a raw image file into qcow2 image.
type QCOW2Pipeline struct {
	BasePipeline
	Compat string

	imgPipeline *LiveImgPipeline
	filename    string
}

// NewQCOW2Pipeline createsa new QCOW2 pipeline. imgPipeline is the pipeline producing the
// raw image. The pipeline name is the name of the new pipeline. Filename is the name
// of the produced qcow2 image.
func NewQCOW2Pipeline(m *Manifest,
	buildPipeline *BuildPipeline,
	imgPipeline *LiveImgPipeline,
	filename string) *QCOW2Pipeline {
	p := &QCOW2Pipeline{
		BasePipeline: NewBasePipeline(m, "qcow2", buildPipeline, nil),
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

func (p *QCOW2Pipeline) serialize() osbuild2.Pipeline {
	pipeline := p.BasePipeline.serialize()

	pipeline.AddStage(osbuild2.NewQEMUStage(
		osbuild2.NewQEMUStageOptions(p.filename,
			osbuild2.QEMUFormatQCOW2,
			osbuild2.QCOW2Options{
				Compat: p.Compat,
			}),
		osbuild2.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.filename),
	))

	return pipeline
}
