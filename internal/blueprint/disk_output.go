package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type diskOutput struct{}

func (t *diskOutput) translate(b *Blueprint) *pipeline.Pipeline {
	packages := [...]string{
		"@core",
		"chrony",
		"firewalld",
		"grub2-pc",
		"kernel",
		"langpacks-en",
		"selinux-policy-targeted",
	}
	excludedPackages := [...]string{
		"dracut-config-rescue",
	}
	p := getCustomF30PackageSet(packages[:], excludedPackages[:])
	addF30LocaleStage(p)
	addF30FSTabStage(p)
	addF30GRUB2Stage(p, b.getKernelCustomization())
	addF30FixBlsStage(p)
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "raw", t.getName())
	return p
}

func (t *diskOutput) getName() string {
	return "disk.img"
}

func (t *diskOutput) getMime() string {
	return "application/octet-stream"
}
