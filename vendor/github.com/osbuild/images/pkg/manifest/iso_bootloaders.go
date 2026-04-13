package manifest

import (
	"bytes"
	"strings"

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

// grub2 PPC64le booting
type Grub2PPC64Boot struct {
	Base

	Platform platform.Platform

	product string
	version string

	ISOLabel string

	KernelOpts []string

	// Default Grub2 menu on the ISO
	DefaultMenu int
}

func NewGrub2PPC64Bootloader(buildPipeline Build, product, version string) *Grub2PPC64Boot {
	p := &Grub2PPC64Boot{
		Base:    NewBase("grub2boot-tree", buildPipeline),
		product: product,
		version: version,
	}
	return p
}

// GetISOBootStages returns the stages and files needed for the grub2 PPC64LE bootloader
func (boot *Grub2PPC64Boot) GetISOBootStages(inputName string, _ *disk.PartitionTable) ([]*osbuild.Stage, []*fsnode.File, error) {
	stages := make([]*osbuild.Stage, 0)

	var grub2config *osbuild.Grub2Config
	if boot.DefaultMenu > 0 {
		grub2config = &osbuild.Grub2Config{
			Default: boot.DefaultMenu,
		}
	}
	options := &osbuild.Grub2ISOLegacyStageOptions{
		Grub2Dir: "boot/grub",
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
		Platform:        "powerpc-ieee1275",
	}
	stages = append(stages, osbuild.NewGrub2ISOLegacyStage(options))

	// Add the bootinfo.txt file which is used by the CHRP boot method to point to grub2
	// It is installed into the /ppc directory
	stages = append(stages, osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{{Path: "/ppc"}},
	}))

	bootinfo, err := fileDataFS.ReadFile("iso/bootinfo.txt")
	if err != nil {
		return nil, nil, err
	}

	f, err := fsnode.NewFile("/ppc/bootinfo.txt", nil, nil, nil, bootinfo)
	if err != nil {
		return nil, nil, err
	}
	stages = append(stages, osbuild.GenFileNodesStages([]*fsnode.File{f})...)

	return stages, []*fsnode.File{f}, nil
}

type S390Boot struct {
	Base

	Platform platform.Platform

	KernelOpts []string
}

func NewS390Bootloader(buildPipeline Build) *S390Boot {
	p := &S390Boot{
		Base: NewBase("s390boot-tree", buildPipeline),
	}
	return p
}

// GetISOBootStages returns the stages and files needed for the S390 bootloader
func (boot *S390Boot) GetISOBootStages(inputName string, _ *disk.PartitionTable) ([]*osbuild.Stage, []*fsnode.File, error) {
	stages := make([]*osbuild.Stage, 0)
	files := make([]*fsnode.File, 0)

	// Copy the configuration files into /images/
	for _, name := range []string{"redhat.exec", "generic.prm", "genericdvd.prm", "generic.ins"} {
		data, err := fileDataFS.ReadFile("iso/s390x/" + name)
		if err != nil {
			return nil, nil, err
		}
		fn, err := fsnode.NewFile("/images/"+name, nil, nil, nil, data)
		if err != nil {
			return nil, nil, err
		}
		stages = append(stages, osbuild.GenFileNodesStages([]*fsnode.File{fn})...)
		files = append(files, fn)
	}

	// cdboot.prm has a @ROOT@ placeholder that needs to be replaced by the KernelOpts
	data, err := fileDataFS.ReadFile("iso/s390x/cdboot.prm")
	if err != nil {
		return nil, nil, err
	}
	cdbootData := bytes.Replace(data, []byte("@ROOT@"), []byte(strings.Join(boot.KernelOpts, " ")), 1)
	fn, err := fsnode.NewFile("/images/cdboot.prm", nil, nil, nil, cdbootData)
	if err != nil {
		return nil, nil, err
	}
	stages = append(stages, osbuild.GenFileNodesStages([]*fsnode.File{fn})...)
	files = append(files, fn)

	// Create the initrd.addrsize file
	addrsizeOptions := &osbuild.CreateaddrsizeStageOptions{
		Initrd:   "/images/pxeboot/initrd.img",
		Addrsize: "/images/initrd.addrsize",
	}
	stages = append(stages, osbuild.NewCreateaddrsizeStage(addrsizeOptions))

	// Create the cdboot.img used by xorrisofs stage
	mkS390ImageOptions := &osbuild.MkS390ImageStageOptions{
		Kernel: "/images/pxeboot/vmlinuz",
		Initrd: "/images/pxeboot/initrd.img",
		Config: "/images/cdboot.prm",
		Image:  "/images/cdboot.img",
	}
	stages = append(stages, osbuild.NewMkS390ImageStage(mkS390ImageOptions))

	return stages, files, nil
}
