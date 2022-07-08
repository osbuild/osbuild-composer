package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// An ISO represents a bootable ISO file created from an
// an existing ISOTreePipeline.
type ISO struct {
	Base
	ISOLinux bool
	Filename string

	treePipeline *ISOTree
}

func NewISO(m *Manifest,
	buildPipeline *Build,
	treePipeline *ISOTree) *ISO {
	p := &ISO{
		Base:         NewBase(m, "bootiso", buildPipeline),
		treePipeline: treePipeline,
		Filename:     "image.iso",
	}
	buildPipeline.addDependent(p)
	if treePipeline.Base.manifest != m {
		panic("tree pipeline from different manifest")
	}
	m.addPipeline(p)
	return p
}

func (p *ISO) getBuildPackages() []string {
	return []string{
		"isomd5sum",
		"xorriso",
	}
}

func (p *ISO) serialize() osbuild2.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild2.NewXorrisofsStage(xorrisofsStageOptions(p.Filename, p.treePipeline.isoLabel, p.ISOLinux), osbuild2.NewXorrisofsStagePipelineTreeInputs(p.treePipeline.Name())))
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
