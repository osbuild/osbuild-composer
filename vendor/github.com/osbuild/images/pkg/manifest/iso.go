package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

type ISOGrub2MenuEntry struct {
	Name   string
	Linux  string
	Initrd string
}

type ISOCustomizations struct {
	// ISO metadata fields
	Label       string
	Preparer    string
	Publisher   string
	Application string

	RootfsType   ISORootfsType
	ErofsOptions osbuild.ErofsStageOptions
	BootType     ISOBootType

	ExcludePaths []string
}

// An ISO represents a bootable ISO file created from an
// an existing ISOTreePipeline.
type ISO struct {
	Base
	filename string

	treePipeline Pipeline

	ISOCustomizations ISOCustomizations
}

func (p ISO) Filename() string {
	return p.filename
}

func (p *ISO) SetFilename(filename string) {
	p.filename = filename
}

func NewISO(buildPipeline Build, treePipeline Pipeline, isoCustomizations ISOCustomizations) *ISO {
	p := &ISO{
		Base:              NewBase("bootiso", buildPipeline),
		treePipeline:      treePipeline,
		filename:          "image.iso",
		ISOCustomizations: isoCustomizations,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *ISO) getBuildPackages(Distro) ([]string, error) {
	return []string{
		"isomd5sum",
		"xorriso",
	}, nil
}

func (p *ISO) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	pipeline.AddStage(osbuild.NewXorrisofsStage(xorrisofsStageOptions(p.Filename(), p.ISOCustomizations), p.treePipeline.Name()))
	pipeline.AddStage(osbuild.NewImplantisomd5Stage(&osbuild.Implantisomd5StageOptions{Filename: p.Filename()}))

	return pipeline, nil
}

func xorrisofsStageOptions(filename string, isoCustomizations ISOCustomizations) *osbuild.XorrisofsStageOptions {
	options := &osbuild.XorrisofsStageOptions{
		Filename: filename,
		VolID:    isoCustomizations.Label,
		SysID:    "LINUX",
		ISOLevel: 3,
		Prep:     isoCustomizations.Preparer,
		Pub:      isoCustomizations.Publisher,
		AppID:    isoCustomizations.Application,
	}

	if len(isoCustomizations.ExcludePaths) > 0 {
		options.Exclude = isoCustomizations.ExcludePaths
	}

	switch isoCustomizations.BootType {
	case Grub2UEFIOnlyISOBoot:
		options.EFI = "images/efiboot.img"
	case SyslinuxISOBoot:
		// Syslinux BIOS ISO creation
		options.EFI = "images/efiboot.img"
		options.Boot = &osbuild.XorrisofsBoot{
			Image:   "isolinux/isolinux.bin",
			Catalog: "isolinux/boot.cat",
		}
		options.IsohybridMBR = "/usr/share/syslinux/isohdpfx.bin"
	case Grub2ISOBoot:
		// grub2 BIOS ISO creation
		options.EFI = "images/efiboot.img"
		options.Boot = &osbuild.XorrisofsBoot{
			Image:   "images/eltorito.img",
			Catalog: "boot.cat",
		}
		options.Grub2MBR = "/usr/lib/grub/i386-pc/boot_hybrid.img"
	case Grub2PPCISOBoot:
		// PPC64LE ISO creation
		options.RationalRock = true
		options.UntranslatedFilenames = true
		options.FullFilenames = true
		options.CHRPBoot = true
		options.VolSet = isoCustomizations.Label
	case S390ISOBoot:
		// S390x ISO creation
		options.Boot = &osbuild.XorrisofsBoot{
			Image:      "images/cdboot.img",
			Catalog:    "images/boot.cat",
			NoBootInfo: true,
		}
	}

	return options
}

func (p *ISO) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-iso9660-image"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
