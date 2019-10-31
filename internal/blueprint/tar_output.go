package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type tarOutput struct{}

func (t *tarOutput) translate(b *Blueprint) *pipeline.Pipeline {
	packages := [...]string{
		"policycoreutils",
		"selinux-policy-targeted",
		"kernel",
		"firewalld",
		"chrony",
		"langpacks-en",
	}
	excludedPackages := [...]string{
		"dracut-config-rescue",
	}
	p := getCustomF30PackageSet(packages[:], excludedPackages[:], b)
	addF30LocaleStage(p)
	addF30GRUB2Stage(p, b.getKernelCustomization())
	addF30FixBlsStage(p)
	addF30SELinuxStage(p)
	addF30TarAssembler(p, t.getName(), "xz")
	return p
}

func (t *tarOutput) getName() string {
	return "root.tar.xz"
}

func (t *tarOutput) getMime() string {
	return "application/x-tar"
}
