package manifest

import (
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
)

// syslinux booting
type ISOLinuxBoot struct {
	Base

	Platform platform.Platform

	product string
	version string

	KernelOpts []string
}

func NewISOLinuxBootloader(buildPipeline Build, product, version string) *ISOLinuxBoot {
	p := &ISOLinuxBoot{
		Base:    NewBase("isolinuxboot-tree", buildPipeline),
		product: product,
		version: version,
	}
	return p
}

// GetISOBootStages returns the stages and files needed for the isolinux bootloader
func (boot *ISOLinuxBoot) GetISOBootStages(inputName string, _ *disk.PartitionTable) ([]*osbuild.Stage, []*fsnode.File, error) {
	options := &osbuild.ISOLinuxStageOptions{
		Product: osbuild.ISOLinuxProduct{
			Name:    boot.product,
			Version: boot.version,
		},
		Kernel: osbuild.ISOLinuxKernel{
			Dir:  "/images/pxeboot",
			Opts: boot.KernelOpts,
		},
		FIPS: boot.Platform.GetFIPSMenu(),
	}

	return []*osbuild.Stage{osbuild.NewISOLinuxStage(options, inputName)}, []*fsnode.File{}, nil
}

// grub2 x86 booting
type Grub2X86Boot struct {
	Base

	Platform platform.Platform

	product string
	version string

	ISOLabel string

	KernelOpts []string

	// Default Grub2 menu on the ISO
	DefaultMenu int
}

func NewGrub2X86Bootloader(buildPipeline Build, product, version string) *Grub2X86Boot {
	p := &Grub2X86Boot{
		Base:    NewBase("grub2boot-tree", buildPipeline),
		product: product,
		version: version,
	}
	return p
}

// GetISOBootStages returns the stages and files needed for the grub2 x86_64 bios bootloader
func (boot *Grub2X86Boot) GetISOBootStages(inputName string, _ *disk.PartitionTable) ([]*osbuild.Stage, []*fsnode.File, error) {
	stages := make([]*osbuild.Stage, 0)

	var grub2config *osbuild.Grub2Config
	if boot.DefaultMenu > 0 {
		grub2config = &osbuild.Grub2Config{
			Default: boot.DefaultMenu,
		}
	}
	options := &osbuild.Grub2ISOLegacyStageOptions{
		Product: osbuild.Product{
			Name:    boot.product,
			Version: boot.version,
		},
		Kernel: osbuild.ISOKernel{
			Dir:  "/images/pxeboot",
			Opts: boot.KernelOpts,
		},
		ISOLabel:        boot.ISOLabel,
		FIPS:            boot.Platform.GetFIPSMenu(),
		Install:         true,
		Test:            true,
		Troubleshooting: true,
		Config:          grub2config,
	}

	stages = append(stages, osbuild.NewGrub2ISOLegacyStage(options))

	// Add a stage to create the eltorito.img file for grub2 BIOS boot support
	stages = append(stages, osbuild.NewGrub2InstStage(osbuild.NewGrub2InstISO9660StageOption("images/eltorito.img", "/boot/grub2")))

	return stages, []*fsnode.File{}, nil
}
