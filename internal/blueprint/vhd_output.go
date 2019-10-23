package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type vhdOutput struct{}

func (t *vhdOutput) translate(b *Blueprint) *pipeline.Pipeline {
	packages := [...]string{
		"@Core",
		"chrony",
		"kernel",
		"selinux-policy-targeted",
		"grub2-pc",
		"langpacks-en",
		"net-tools",
		"ntfsprogs",
		"WALinuxAgent",
		"libxcrypt-compat",
	}
	p := getCustomF30PackageSet(packages[:], []string{})
	addF30LocaleStage(p)
	addF30FSTabStage(p)
	addF30GRUB2Stage(p)
	addF30SELinuxStage(p)
	addF30FixBlsStage(p)
	addF30QemuAssembler(p, "qcow2", t.getName())
	return p
}

func (t *vhdOutput) getName() string {
	return "image.vhd"
}

func (t *vhdOutput) getMime() string {
	return "application/x-vhd"
}
