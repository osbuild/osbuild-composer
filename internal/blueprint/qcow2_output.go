package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type qcow2Output struct{}

func (t *qcow2Output) translate(b *Blueprint) *pipeline.Pipeline {
	packages := [...]string{
		"kernel-core",
		"@Fedora Cloud Server",
		"chrony",
		"polkit",
		"systemd-udev",
		"selinux-policy-targeted",
		"grub2-pc",
		"langpacks-en",
	}
	excludedPackages := [...]string{
		"dracut-config-rescue",
		"etables",
		"firewalld",
		"gobject-introspection",
		"plymouth",
	}
	p := getCustomF30PackageSet(packages[:], excludedPackages[:])
	addF30LocaleStage(p)
	addF30FSTabStage(p)
	addF30GRUB2Stage(p, b.getKernelCustomization())
	addF30FixBlsStage(p)
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "qcow2", t.getName())

	return p
}

func (t *qcow2Output) getName() string {
	return "image.qcow2"
}

func (t *qcow2Output) getMime() string {
	return "application/x-qemu-disk"
}
