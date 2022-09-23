package manifest

import (
	"fmt"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/platform"
)

type EFIBootTree struct {
	Base

	Platform platform.Platform

	anacondaPipeline *Anaconda

	UEFIVendor string
	ISOLabel   string
	KSPath     string
}

func NewEFIBootTree(m *Manifest, buildPipeline *Build, anacondaPipeline *Anaconda) *EFIBootTree {
	p := &EFIBootTree{
		Base:             NewBase(m, "efiboot-tree", buildPipeline),
		anacondaPipeline: anacondaPipeline,
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *EFIBootTree) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	arch := p.Platform.GetArch().String()
	var architectures []string
	if arch == distro.X86_64ArchName {
		architectures = []string{"X64"}
	} else if arch == distro.Aarch64ArchName {
		architectures = []string{"AA64"}
	} else {
		panic("unsupported architecture")
	}

	grubOptions := &osbuild.GrubISOStageOptions{
		Product: osbuild.Product{
			Name:    p.anacondaPipeline.product,
			Version: p.anacondaPipeline.version,
		},
		Kernel: osbuild.ISOKernel{
			Dir:  "/images/pxeboot",
			Opts: []string{fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", p.ISOLabel, p.KSPath)},
		},
		ISOLabel:      p.ISOLabel,
		Architectures: architectures,
		Vendor:        p.UEFIVendor,
	}
	grub2Stage := osbuild.NewGrubISOStage(grubOptions)
	pipeline.AddStage(grub2Stage)
	return pipeline
}
