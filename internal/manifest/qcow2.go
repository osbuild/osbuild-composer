package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
)

// A QCOW2 turns a raw image file into qcow2 image.
type QCOW2 struct {
	Base
	Filename string
	Compat   string

	Manifest     *Manifest
	ImgFilename  string
	PipelineName string
}

// NewQCOW2 creates a new QCOW2 pipeline. imgPipelineName is the name of the
// pipeline producing the raw image and imgFilename is the file name.
func NewQCOW2(m *Manifest,
	buildPipeline *Build,
	manifest *Manifest,
	imgPipelineName string,
	imgFilename string) *QCOW2 {
	p := &QCOW2{
		Base:         NewBase(m, "qcow2", buildPipeline),
		Manifest:     manifest,
		Filename:     "image.qcow2",
		ImgFilename:  imgFilename,
		PipelineName: imgPipelineName,
	}
	if manifest != m {
		panic("live image pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *QCOW2) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewQEMUStage(
		osbuild.NewQEMUStageOptions(p.Filename,
			osbuild.QEMUFormatQCOW2,
			osbuild.QCOW2Options{
				Compat: p.Compat,
			}),
		osbuild.NewQemuStagePipelineFilesInputs(p.PipelineName, p.ImgFilename),
	))

	return pipeline
}

func (p *QCOW2) getBuildPackages(Distro) []string {
	return []string{"qemu-img"}
}

func (p *QCOW2) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-qemu-disk"
	return artifact.New(p.Name(), p.Filename, &mimeType)
}
