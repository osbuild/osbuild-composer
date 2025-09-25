package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

// A VPC turns a raw image file into qemu-based image format, such as vhd.
type VPC struct {
	Base
	filename string

	ForceSize *bool

	imgPipeline FilePipeline
}

func (p VPC) Filename() string {
	return p.filename
}

func (p *VPC) SetFilename(filename string) {
	p.filename = filename
}

// NewVPC createsa new Qemu pipeline. imgPipeline is the pipeline producing the
// raw image. The pipeline name is the name of the new pipeline. Filename is the name
// of the produced image.
func NewVPC(buildPipeline Build, imgPipeline FilePipeline) *VPC {
	p := &VPC{
		Base:        NewBase("vpc", buildPipeline),
		imgPipeline: imgPipeline,
		filename:    "image.vhd",
	}
	// vpc can run outside the build pipeline for e.g. "bib"
	if buildPipeline != nil {
		buildPipeline.addDependent(p)
	} else {
		imgPipeline.Manifest().addPipeline(p)
	}
	return p
}

func (p *VPC) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	formatOptions := osbuild.VPCOptions{ForceSize: p.ForceSize}

	pipeline.AddStage(osbuild.NewQEMUStage(
		osbuild.NewQEMUStageOptions(p.Filename(), osbuild.QEMUFormatVPC, formatOptions),
		osbuild.NewQemuStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.Filename()),
	))

	return pipeline, nil
}

func (p *VPC) getBuildPackages(Distro) ([]string, error) {
	return []string{"qemu-img"}, nil
}

func (p *VPC) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-vhd"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
