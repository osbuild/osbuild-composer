package platform

type S390X struct {
	BasePlatform
	Zipl bool
}

func (p *S390X) GetArch() Arch {
	return ARCH_S390X
}

func (p *S390X) GetZiplSupport() bool {
	return p.Zipl
}

func (p *S390X) GetPackages() []string {
	packages := p.BasePlatform.FirmwarePackages
	// TODO: should these packages be present also in images not intended for booting?
	packages = append(packages,
		"dracut-config-generic",
		"s390utils-base",
		"s390utils-core",
	)
	return packages
}

func (p *S390X) GetBuildPackages() []string {
	// TODO: should these packages be present also in images not intended for booting?
	return []string{
		"s390utils-base",
	}
}
