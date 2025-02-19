package platform

import (
	"github.com/osbuild/images/pkg/arch"
)

type RISCV64 struct {
	BasePlatform
}

func (p *RISCV64) GetArch() arch.Arch {
	return arch.ARCH_RISCV64
}

func (p *RISCV64) GetPackages() []string {
	packages := p.BasePlatform.FirmwarePackages

	return packages
}

func (p *RISCV64) GetBuildPackages() []string {
	var packages []string

	return packages
}
