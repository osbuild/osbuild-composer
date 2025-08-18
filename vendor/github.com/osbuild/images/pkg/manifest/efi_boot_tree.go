package manifest

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
)

type EFIBootTree struct {
	Base

	Platform platform.Platform

	product string
	version string

	UEFIVendor string
	ISOLabel   string

	KernelOpts []string

	// Default Grub2 menu on the ISO
	DefaultMenu int
}

func NewEFIBootTree(buildPipeline Build, product, version string) *EFIBootTree {
	p := &EFIBootTree{
		Base:    NewBase("efiboot-tree", buildPipeline),
		product: product,
		version: version,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *EFIBootTree) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	a := p.Platform.GetArch().String()
	var architectures []string
	if a == arch.ARCH_X86_64.String() {
		architectures = []string{"X64"}
	} else if a == arch.ARCH_AARCH64.String() {
		architectures = []string{"AA64"}
	} else {
		panic("unsupported architecture")
	}

	var grub2config *osbuild.Grub2Config
	if p.DefaultMenu > 0 {
		grub2config = &osbuild.Grub2Config{
			Default: p.DefaultMenu,
		}
	}

	grubOptions := &osbuild.GrubISOStageOptions{
		Product: osbuild.Product{
			Name:    p.product,
			Version: p.version,
		},
		Kernel: osbuild.ISOKernel{
			Dir:  "/images/pxeboot",
			Opts: p.KernelOpts,
		},
		ISOLabel:      p.ISOLabel,
		Architectures: architectures,
		Vendor:        p.UEFIVendor,
		FIPS:          p.Platform.GetFIPSMenu(),
		Config:        grub2config,
	}
	grub2Stage := osbuild.NewGrubISOStage(grubOptions)
	pipeline.AddStage(grub2Stage)
	return pipeline
}
