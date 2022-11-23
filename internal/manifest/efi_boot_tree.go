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

	product string
	version string

	UEFIVendor string
	ISOLabel   string
	KSPath     string

	KernelOpts []string
}

func NewEFIBootTree(m *Manifest, buildPipeline *Build, product, version string) *EFIBootTree {
	p := &EFIBootTree{
		Base:    NewBase(m, "efiboot-tree", buildPipeline),
		product: product,
		version: version,
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

	kernelOpts := []string{}

	if p.KSPath != "" {
		kernelOpts = append(kernelOpts, fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", p.ISOLabel, p.KSPath))
	} else {
		kernelOpts = append(kernelOpts, fmt.Sprintf("inst.stage2=hd:LABEL=%s", p.ISOLabel))
	}

	if len(p.KernelOpts) > 0 {
		kernelOpts = append(kernelOpts, p.KernelOpts...)
	}

	grubOptions := &osbuild.GrubISOStageOptions{
		Product: osbuild.Product{
			Name:    p.product,
			Version: p.version,
		},
		Kernel: osbuild.ISOKernel{
			Dir:  "/images/pxeboot",
			Opts: kernelOpts,
		},
		ISOLabel:      p.ISOLabel,
		Architectures: architectures,
		Vendor:        p.UEFIVendor,
	}
	grub2Stage := osbuild.NewGrubISOStage(grubOptions)
	pipeline.AddStage(grub2Stage)
	return pipeline
}
