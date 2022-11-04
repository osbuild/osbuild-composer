package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
)

// The XZ pipeline compresses a raw image file using xz.
type XZ struct {
	Base
	Filename string

	imgPipeline Pipeline
}

// NewXZ creates a new XZ pipeline. imgPipeline is the pipeline producing the
// raw image that will be xz compressed.
func NewXZ(m *Manifest,
	buildPipeline *Build,
	imgPipeline Pipeline) *XZ {
	p := &XZ{
		Base:        NewBase(m, "xz", buildPipeline),
		Filename:    "image.xz",
		imgPipeline: imgPipeline,
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *XZ) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewXzStage(
		osbuild.NewXzStageOptions(p.Filename),
		osbuild.NewFilesInputs(osbuild.NewFilesInputReferencesPipeline(p.imgPipeline.Name(), p.imgPipeline.Export().Filename())),
	))

	return pipeline
}

func (p *XZ) getBuildPackages() []string {
	return []string{"xz"}
}

func (p *XZ) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/xz"
	return artifact.New(p.Name(), p.Filename, &mimeType)
}
