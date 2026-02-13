package manifest

import (
	"fmt"

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

	// Default Grub2 menu timeout on the ISO
	MenuTimeout *int

	// Potentially custom menu entries
	MenuEntries []ISOGrub2MenuEntry

	DisableTestEntry            bool
	DisableTroubleshootingEntry bool
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

func (p *EFIBootTree) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	a := p.Platform.GetArch().String()
	var architectures []string
	if a == arch.ARCH_X86_64.String() {
		architectures = []string{"X64"}
	} else if a == arch.ARCH_AARCH64.String() {
		architectures = []string{"AA64"}
	} else {
		return osbuild.Pipeline{}, fmt.Errorf("EFIBootTree: unsupported architecture %q", a)
	}

	var grub2config *osbuild.Grub2Config

	if p.DefaultMenu > 0 {
		grub2config = &osbuild.Grub2Config{
			Default: p.DefaultMenu,
		}
	}

	if p.MenuTimeout != nil {
		if grub2config == nil {
			grub2config = &osbuild.Grub2Config{
				Timeout: *p.MenuTimeout,
			}
		} else {
			grub2config.Timeout = *p.MenuTimeout
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
		ISOLabel:        p.ISOLabel,
		Architectures:   architectures,
		Vendor:          p.UEFIVendor,
		FIPS:            p.Platform.GetFIPSMenu(),
		Install:         true,
		Test:            !p.DisableTestEntry,
		Troubleshooting: !p.DisableTroubleshootingEntry,
		Config:          grub2config,
	}

	// If any menu entries are defined we turn off all default
	// entries and instead append our own
	// entries only
	if len(p.MenuEntries) > 0 {
		grubOptions.Troubleshooting = false
		grubOptions.Test = false
		grubOptions.Install = false

		for _, entry := range p.MenuEntries {
			grubOptions.Custom = append(grubOptions.Custom, osbuild.GrubISOCustomEntryOptions{
				Name:   entry.Name,
				Linux:  entry.Linux,
				Initrd: entry.Initrd,
			})
		}
	}

	grub2Stage := osbuild.NewGrubISOStage(grubOptions)
	pipeline.AddStage(grub2Stage)
	return pipeline, nil
}
