package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

// The Gzip pipeline compresses a raw image file using gzip.
type Gzip struct {
	Base
	filename string

	imgPipeline FilePipeline
}

func (p Gzip) Filename() string {
	return p.filename
}

func (p *Gzip) SetFilename(filename string) {
	p.filename = filename
}

// NewGzip creates a new Gzip pipeline. imgPipeline is the pipeline producing the
// raw image that will be gzip compressed.
func NewGzip(buildPipeline Build, imgPipeline FilePipeline) *Gzip {
	p := &Gzip{
		Base:        NewBase("gzip", buildPipeline),
		filename:    "image.gz",
		imgPipeline: imgPipeline,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *Gzip) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	pipeline.AddStage(osbuild.NewGzipStage(
		osbuild.NewGzipStageOptions(p.Filename()),
		osbuild.NewGzipStageInputs(osbuild.NewFilesInputPipelineObjectRef(p.imgPipeline.Name(), p.imgPipeline.Export().Filename(), nil)),
	))

	return pipeline, nil
}

func (p *Gzip) getBuildPackages(Distro) ([]string, error) {
	return []string{"gzip"}, nil
}

func (p *Gzip) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/gzip"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
