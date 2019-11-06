package fedora30

import (
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

type vmdkOutput struct{}

func (t *vmdkOutput) translate(b *blueprint.Blueprint) (*pipeline.Pipeline, error) {
	packages := [...]string{
		"@core",
		"chrony",
		"firewalld",
		"grub2-pc",
		"kernel",
		"langpacks-en",
		"open-vm-tools",
		"selinux-policy-targeted",
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
	addF30QemuAssembler(p, "vmdk", t.getName())

	if b.Customizations != nil {
		err := customizeAll(p, b.Customizations)
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (t *vmdkOutput) getName() string {
	return "disk.vmdk"
}

func (t *vmdkOutput) getMime() string {
	return "application/x-vmdk"
}
