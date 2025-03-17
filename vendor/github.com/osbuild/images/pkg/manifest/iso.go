package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

// An ISO represents a bootable ISO file created from an
// an existing ISOTreePipeline.
type ISO struct {
	Base
	ISOBoot  ISOBootType
	filename string

	treePipeline Pipeline
	isoLabel     string
}

func (p ISO) Filename() string {
	return p.filename
}

func (p *ISO) SetFilename(filename string) {
	p.filename = filename
}

func NewISO(buildPipeline Build, treePipeline Pipeline, isoLabel string) *ISO {
	p := &ISO{
		Base:         NewBase("bootiso", buildPipeline),
		treePipeline: treePipeline,
		filename:     "image.iso",
		isoLabel:     isoLabel,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *ISO) getBuildPackages(Distro) []string {
	return []string{
		"isomd5sum",
		"xorriso",
	}
}

func (p *ISO) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewXorrisofsStage(xorrisofsStageOptions(p.Filename(), p.isoLabel, p.ISOBoot), p.treePipeline.Name()))
	pipeline.AddStage(osbuild.NewImplantisomd5Stage(&osbuild.Implantisomd5StageOptions{Filename: p.Filename()}))

	return pipeline
}

func xorrisofsStageOptions(filename, isolabel string, isoboot ISOBootType) *osbuild.XorrisofsStageOptions {
	options := &osbuild.XorrisofsStageOptions{
		Filename: filename,
		VolID:    isolabel,
		SysID:    "LINUX",
		EFI:      "images/efiboot.img",
		ISOLevel: 3,
	}

	if isoboot == SyslinuxISOBoot {
		// Syslinux BIOS ISO creation
		options.Boot = &osbuild.XorrisofsBoot{
			Image:   "isolinux/isolinux.bin",
			Catalog: "isolinux/boot.cat",
		}
		options.IsohybridMBR = "/usr/share/syslinux/isohdpfx.bin"
	} else if isoboot == Grub2ISOBoot {
		// grub2 BIOS ISO creation
		options.Boot = &osbuild.XorrisofsBoot{
			Image:   "images/eltorito.img",
			Catalog: "boot.cat",
		}
		options.Grub2MBR = "/usr/lib/grub/i386-pc/boot_hybrid.img"
	}

	return options
}

func (p *ISO) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-iso9660-image"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
