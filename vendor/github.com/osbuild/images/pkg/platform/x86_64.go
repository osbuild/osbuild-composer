package platform

import (
	"github.com/osbuild/images/pkg/arch"
)

type X86 struct {
	BasePlatform
	BIOS       bool
	UEFIVendor string
	Bootloader Bootloader
}

func (p *X86) GetArch() arch.Arch {
	return arch.ARCH_X86_64
}

func (p *X86) GetBIOSPlatform() string {
	if p.BIOS {
		return "i386-pc"
	}
	return ""
}

func (p *X86) GetUEFIVendor() string {
	return p.UEFIVendor
}

func (p *X86) GetPackages() []string {
	packages := p.BasePlatform.FirmwarePackages

	switch p.GetBootloader() {
	case BOOTLOADER_GRUB2:
		if p.BIOS {
			packages = append(packages,
				"dracut-config-generic",
				"grub2-pc")
		}

		if p.UEFIVendor != "" {
			packages = append(packages,
				"dracut-config-generic",
				"efibootmgr",
				"grub2-efi-x64",
				"shim-x64")
		}
	case BOOTLOADER_UKI:
		packages = append(packages,
			"efibootmgr",
			"kernel-uki-virt-addons", // provides useful cmdline utilities for the UKI
			"shim-x64",
			"uki-direct",
		)
	}

	return packages
}

func (p *X86) GetBuildPackages() []string {
	packages := []string{}
	if p.BIOS {
		packages = append(packages, "grub2-pc")
	}
	return packages
}

func (p *X86) GetBootloader() Bootloader {
	if p.Bootloader == BOOTLOADER_NONE {
		return BOOTLOADER_GRUB2
	}
	return p.Bootloader
}
