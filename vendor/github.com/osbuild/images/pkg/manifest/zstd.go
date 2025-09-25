package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

// The Zstd pipeline compresses a raw image file using zstd.
type Zstd struct {
	Base
	filename string

	imgPipeline FilePipeline
}

func (p Zstd) Filename() string {
	return p.filename
}

func (p *Zstd) SetFilename(filename string) {
	p.filename = filename
}

// NewZstd creates a new Zstd pipeline. imgPipeline is the pipeline producing the
// raw image that will be zstd compressed.
func NewZstd(buildPipeline Build, imgPipeline FilePipeline) *Zstd {
	p := &Zstd{
		Base:        NewBase("zstd", buildPipeline),
		filename:    "image.zst",
		imgPipeline: imgPipeline,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *Zstd) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	pipeline.AddStage(osbuild.NewZstdStage(
		osbuild.NewZstdStageOptions(p.Filename()),
		osbuild.NewZstdStageInputs(osbuild.NewFilesInputPipelineObjectRef(p.imgPipeline.Name(), p.imgPipeline.Export().Filename(), nil)),
	))

	return pipeline, nil
}

func (p *Zstd) getBuildPackages(Distro) ([]string, error) {
	return []string{"zstd"}, nil
}

func (p *Zstd) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/zstd"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
