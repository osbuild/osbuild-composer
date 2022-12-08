package platform

type S390X struct {
	BasePlatform
	BIOS bool
}

func (p *S390X) GetArch() Arch {
	return ARCH_S390X
}

func (p *S390X) GetPackages() []string {
	return []string{
		"dracut-config-generic",
		"s390utils-base",
		"s390utils-core",
	}
}

func (p *S390X) GetBuildPackages() []string {
	return []string{
		"s390utils-base",
	}
}
