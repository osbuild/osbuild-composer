package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type amiOutput struct{}

func (t *amiOutput) translate(b *Blueprint) *pipeline.Pipeline {
	packages := [...]string{
		"@Core",
		"chrony",
		"kernel",
		"selinux-policy-targeted",
		"grub2-pc",
		"langpacks-en",
		"libxcrypt-compat",
		"xfsprogs",
		"cloud-init",
		"checkpolicy",
		"net-tools",
	}
	p := getCustomF30PackageSet(packages[:], []string{})
	addF30FixBlsStage(p)
	addF30LocaleStage(p)
	addF30FSTabStage(p)
	addF30GRUB2Stage(p)
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "qcow2", t.getName())
	return p
}

func (t *amiOutput) getName() string {
	return "image.ami"
}

func (t *amiOutput) getMime() string {
	return "application/x-qemu-disk"
}
