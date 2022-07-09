package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// A QCOW2 turns a raw image file into qcow2 image.
type QCOW2 struct {
	Base
	Compat string

	imgPipeline *RawImage
	filename    string
}

// NewQCOW2 createsa new QCOW2 pipeline. imgPipeline is the pipeline producing the
// raw image. The pipeline name is the name of the new pipeline. Filename is the name
// of the produced qcow2 image.
func NewQCOW2(m *Manifest,
	buildPipeline *Build,
	imgPipeline *RawImage,
	filename string) *QCOW2 {
	p := &QCOW2{
		Base:        NewBase(m, "qcow2", buildPipeline),
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

func (p *QCOW2) serialize() osbuild2.Pipeline {
	pipeline := p.Base.serialize()

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

func (p *QCOW2) getBuildPackages() []string {
	return []string{"qemu-img"}
}
