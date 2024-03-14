package manifest

import (
	"fmt"

	"github.com/osbuild/images/pkg/osbuild"
)

// A OVF copies a vmdk image to it's own tree and generates an OVF descriptor
type OVF struct {
	Base

	imgPipeline *VMDK
}

// NewOVF creates a new OVF pipeline. imgPipeline is the pipeline producing the vmdk image.
func NewOVF(buildPipeline Build, imgPipeline *VMDK) *OVF {
	p := &OVF{
		Base:        NewBase("ovf", buildPipeline),
		imgPipeline: imgPipeline,
	}
	// See similar logic in qcow2 to run on the host
	if buildPipeline != nil {
		buildPipeline.addDependent(p)
	} else {
		imgPipeline.Manifest().addPipeline(p)
	}
	return p
}

func (p *OVF) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	inputName := "vmdk-tree"
	pipeline.AddStage(osbuild.NewCopyStageSimple(
		&osbuild.CopyStageOptions{
			Paths: []osbuild.CopyStagePath{
				{
					From: fmt.Sprintf("input://%s/%s", inputName, p.imgPipeline.Export().Filename()),
					To:   "tree:///",
				},
			},
		},
		osbuild.NewPipelineTreeInputs(inputName, p.imgPipeline.Name()),
	))

	pipeline.AddStage(osbuild.NewOVFStage(&osbuild.OVFStageOptions{
		Vmdk: p.imgPipeline.Filename(),
	}))

	return pipeline
}

func (p *OVF) getBuildPackages(Distro) []string {
	return []string{"qemu-img"}
}
