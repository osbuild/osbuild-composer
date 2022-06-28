package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// An ISOPipeline represents a bootable ISO file created from an
// an existing ISOTreePipeline.
type ISOPipeline struct {
	BasePipeline
	ISOLinux bool

	treePipeline *ISOTreePipeline
	filename     string
}

func NewISOPipeline(buildPipeline *BuildPipeline, treePipeline *ISOTreePipeline, filename string) ISOPipeline {
	return ISOPipeline{
		BasePipeline: NewBasePipeline("bootiso", buildPipeline, nil),
		treePipeline: treePipeline,
		filename:     filename,
	}
}

func (p ISOPipeline) serialize() osbuild2.Pipeline {
	pipeline := p.BasePipeline.serialize()

	pipeline.AddStage(osbuild2.NewXorrisofsStage(xorrisofsStageOptions(p.filename, p.treePipeline.isoLabel, p.ISOLinux), osbuild2.NewXorrisofsStagePipelineTreeInputs(p.treePipeline.Name())))
	pipeline.AddStage(osbuild2.NewImplantisomd5Stage(&osbuild2.Implantisomd5StageOptions{Filename: p.filename}))

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
