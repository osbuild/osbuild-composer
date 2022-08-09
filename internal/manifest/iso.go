package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
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

func (p *ISO) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewXorrisofsStage(xorrisofsStageOptions(p.Filename, p.treePipeline.isoLabel, p.ISOLinux), osbuild.NewXorrisofsStagePipelineTreeInputs(p.treePipeline.Name())))
	pipeline.AddStage(osbuild.NewImplantisomd5Stage(&osbuild.Implantisomd5StageOptions{Filename: p.Filename}))

	return pipeline
}

func xorrisofsStageOptions(filename, isolabel string, isolinux bool) *osbuild.XorrisofsStageOptions {
	options := &osbuild.XorrisofsStageOptions{
		Filename: filename,
		VolID:    isolabel,
		SysID:    "LINUX",
		EFI:      "images/efiboot.img",
		ISOLevel: 3,
	}

	if isolinux {
		options.Boot = &osbuild.XorrisofsBoot{
			Image:   "isolinux/isolinux.bin",
			Catalog: "isolinux/boot.cat",
		}

		options.IsohybridMBR = "/usr/share/syslinux/isohdpfx.bin"
	}

	return options
}

func (p *ISO) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-iso9660-image"
	return artifact.New(p.Name(), p.Filename, &mimeType)
}
