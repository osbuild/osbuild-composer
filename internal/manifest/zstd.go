package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
)

// The zstd pipeline compresses a raw image file using zstd.
type Zstd struct {
	Base
	Filename string

	imgPipeline Pipeline
}

// NewZstd creates a new zstd pipeline. imgPipeline is the pipeline producing the
// raw image that will be zstd compressed.
func NewZstd(m *Manifest,
	buildPipeline *Build,
	imgPipeline Pipeline) *Zstd {
	p := &Zstd{
		Base:        NewBase(m, "zstd", buildPipeline),
		Filename:    "image.zstd",
		imgPipeline: imgPipeline,
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *Zstd) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewZstdStage(
		osbuild.NewZstdStageOptions(p.Filename),
		osbuild.NewZstdStageInputs(osbuild.NewFilesInputPipelineObjectRef(p.imgPipeline.Name(), p.imgPipeline.Export().Filename(), nil)),
	))

	return pipeline
}

func (p *Zstd) getBuildPackages() []string {
	return []string{"zstd"}
}

func (p *Zstd) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/zstd"
	return artifact.New(p.Name(), p.Filename, &mimeType)
}
