package platform

import (
	"github.com/osbuild/images/pkg/arch"
)

type RISCV64 struct {
	BasePlatform
	UEFIVendor string
}

func (p *RISCV64) GetArch() arch.Arch {
	return arch.ARCH_RISCV64
}

func (p *RISCV64) GetPackages() []string {
	packages := p.BasePlatform.FirmwarePackages

	if p.UEFIVendor != "" {
		packages = append(packages,
			// XXX: this is needed to get a generic bootkernel,
			// this should probably be part of any bootable img
			// packagelist
			"dracut-config-generic",
			"grub2-efi-riscv64",
			"grub2-efi-riscv64-modules",
			"shim-unsigned-riscv64",
		)
	}

	return packages
}

func (p *RISCV64) GetBuildPackages() []string {
	var packages []string

	return packages
}

func (p *RISCV64) GetUEFIVendor() string {
	return p.UEFIVendor
}
