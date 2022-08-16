package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
)

// The XZ pipeline compresses a raw image file using xz.
type XZ struct {
	Base
	Filename string

	imgPipeline *RawOSTreeImage
}

// NewXZ creates a new XZ pipeline. imgPipeline is the pipeline producing the
// raw image. Filename is the name of the produced archive.
func NewXZ(m *Manifest,
	buildPipeline *Build,
	imgPipeline *RawOSTreeImage) *XZ {
	p := &XZ{
		Base:        NewBase(m, "xz", buildPipeline),
		imgPipeline: imgPipeline,
		Filename:    "image.xz",
	}
	if imgPipeline.Base.manifest != m {
		panic("live image pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *XZ) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewXzStage(
		osbuild.NewXzStageOptions(p.Filename),
		osbuild.NewFilesInputs(osbuild.NewFilesInputReferencesPipeline(p.imgPipeline.Name(), p.imgPipeline.Filename)),
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
