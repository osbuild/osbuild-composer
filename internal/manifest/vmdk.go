package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
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

func (p *VMDK) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewQEMUStage(
		osbuild.NewQEMUStageOptions(p.Filename, osbuild.QEMUFormatVMDK, osbuild.VMDKOptions{
			Subformat: osbuild.VMDKSubformatStreamOptimized,
		}),
		osbuild.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.Filename),
	))

	return pipeline
}

func (p *VMDK) getBuildPackages() []string {
	return []string{"qemu-img"}
}

func (p *VMDK) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-vmdk"
	return artifact.New(p.Name(), p.Filename, &mimeType)
}
