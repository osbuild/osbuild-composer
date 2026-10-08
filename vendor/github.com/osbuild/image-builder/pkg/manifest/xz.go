package manifest

import (
	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

// The XZ pipeline compresses a raw image file using xz.
type XZ struct {
	Base
	filename string

	imgPipeline FilePipeline
}

func (p XZ) Filename() string {
	return p.filename
}

func (p *XZ) SetFilename(filename string) {
	p.filename = filename
}

// NewXZ creates a new XZ pipeline. imgPipeline is the pipeline producing the
// raw image that will be xz compressed. If name is empty, "xz" is used.
func NewXZ(buildPipeline Build, imgPipeline FilePipeline, name string) *XZ {
	if name == "" {
		name = "xz"
	}
	p := &XZ{
		Base:        NewBase(name, buildPipeline),
		filename:    "image.xz",
		imgPipeline: imgPipeline,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *XZ) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	pipeline.AddStage(osbuild.NewXzStage(
		osbuild.NewXzStageOptions(p.Filename()),
		osbuild.NewXzStageInputs(osbuild.NewFilesInputPipelineObjectRef(p.imgPipeline.Name(), p.imgPipeline.Export().Filename(), nil)),
	))

	return pipeline, nil
}

func (p *XZ) getBuildPackages(Distro) ([]string, error) {
	return []string{"xz"}, nil
}

func (p *XZ) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/xz"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
