package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// A VMDK turns a raw image file into vmdk image.
type VMDK struct {
	Base
	Filename string

	imgPipeline *RawImage
}

// NewVMDK creates a new VMDK pipeline. imgPipeline is the pipeline producing the
// raw image. Filename is the name of the produced image.
func NewVMDK(m *Manifest,
	buildPipeline *Build,
	imgPipeline *RawImage) *VMDK {
	p := &VMDK{
		Base:        NewBase(m, "vmdk", buildPipeline),
		imgPipeline: imgPipeline,
		Filename:    "image.vmdk",
	}
	if imgPipeline.Base.manifest != m {
		panic("live image pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *VMDK) serialize() osbuild2.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild2.NewQEMUStage(
		osbuild2.NewQEMUStageOptions(p.Filename, osbuild2.QEMUFormatVMDK, osbuild2.VMDKOptions{
			Subformat: osbuild2.VMDKSubformatStreamOptimized,
		}),
		osbuild2.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.Filename),
	))

	return pipeline
}

func (p *VMDK) getBuildPackages() []string {
	return []string{"qemu-img"}
}
