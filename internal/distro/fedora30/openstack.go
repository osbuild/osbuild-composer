package fedora30

import (
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

type openstackOutput struct{}

func (t *openstackOutput) translate(b *blueprint.Blueprint) (*pipeline.Pipeline, error) {
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
	p := getCustomF30PackageSet(packages[:], excludedPackages[:], b)
	addF30LocaleStage(p)
	addF30FSTabStage(p)
	addF30GRUB2Stage(p, b.GetKernelCustomization())
	addF30FixBlsStage(p)
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "qcow2", t.getName())

	if b.Customizations != nil {
		err := customizeAll(p, b.Customizations)
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (t *openstackOutput) getName() string {
	return "image.qcow2"
}

func (t *openstackOutput) getMime() string {
	return "application/x-qemu-disk"
}
