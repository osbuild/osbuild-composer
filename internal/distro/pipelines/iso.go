package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type ISOPipeline struct {
	Pipeline
	treePipeline *ISOTreePipeline
	Filename     string
	ISOLinux     bool
}

func NewISOPipeline(buildPipeline *BuildPipeline, treePipeline *ISOTreePipeline) ISOPipeline {
	return ISOPipeline{
		Pipeline:     New("bootiso", buildPipeline, nil),
		treePipeline: treePipeline,
	}
}

func (p ISOPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	pipeline.AddStage(osbuild2.NewXorrisofsStage(xorrisofsStageOptions(p.Filename, p.treePipeline.ISOLabel(), p.ISOLinux), osbuild2.NewXorrisofsStagePipelineTreeInputs(p.treePipeline.Name())))
	pipeline.AddStage(osbuild2.NewImplantisomd5Stage(&osbuild2.Implantisomd5StageOptions{Filename: p.Filename}))

	return pipeline
}

func xorrisofsStageOptions(filename, isolabel string, isolinux bool) *osbuild2.XorrisofsStageOptions {
	options := &osbuild2.XorrisofsStageOptions{
		Filename: filename,
		VolID:    isolabel,
		SysID:    "LINUX",
		EFI:      "images/efiboot.img",
		ISOLevel: 3,
	}

	if isolinux {
		options.Boot = &osbuild2.XorrisofsBoot{
			Image:   "isolinux/isolinux.bin",
			Catalog: "isolinux/boot.cat",
		}

		options.IsohybridMBR = "/usr/share/syslinux/isohdpfx.bin"
	}

	return options
}
