package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type openstackOutput struct{}

func (t *openstackOutput) translate(b *Blueprint) *pipeline.Pipeline {
	packages := [...]string{
		"@Core",
		"chrony",
		"kernel",
		"selinux-policy-targeted",
		"grub2-pc",
		"spice-vdagent",
		"qemu-guest-agent",
		"xen-libs",
		"langpacks-en",
		"cloud-init",
		"libdrm",
	}
	excludedPackages := [...]string{
		"dracut-config-rescue",
	}
	p := getCustomF30PackageSet(packages[:], excludedPackages[:])
	addF30LocaleStage(p)
	addF30FSTabStage(p)
	addF30GRUB2Stage(p, b.getKernelCustomization())
	addF30SELinuxStage(p)
	addF30FixBlsStage(p)
	addF30QemuAssembler(p, "qcow2", t.getName())
	return p
}

func (t *openstackOutput) getName() string {
	return "image.qcow2"
}

func (t *openstackOutput) getMime() string {
	return "application/x-qemu-disk"
}
