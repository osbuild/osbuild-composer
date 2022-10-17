package platform

type PPC64LE struct {
	BasePlatform
	BIOS bool
}

func (p *PPC64LE) GetArch() Arch {
	return ARCH_PPC64LE
}

func (p *PPC64LE) GetBIOSPlatform() string {
	if p.BIOS {
		return "powerpc-ieee1275"
	}
	return ""
}

func (p *PPC64LE) GetPackages() []string {
	return []string{
		"dracut-config-generic",
		"powerpc-utils",
		"grub2-ppc64le",
		"grub2-ppc64le-modules",
	}
}

func (p *PPC64LE) GetBuildPackages() []string {
	return []string{
		"grub2-ppc64le",
		"grub2-ppc64le-modules",
	}
}
