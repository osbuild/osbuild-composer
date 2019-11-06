package fedora30

import (
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

type amiOutput struct{}

func (t *amiOutput) translate(b *blueprint.Blueprint) (*pipeline.Pipeline, error) {
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
	excludedPackages := [...]string{
		"dracut-config-rescue",
	}
	p := getCustomF30PackageSet(packages[:], excludedPackages[:], b)
	addF30FixBlsStage(p)
	addF30LocaleStage(p)
	addF30FSTabStage(p)
	addF30GRUB2Stage(p, nil)
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

func (t *amiOutput) getName() string {
	return "image.ami"
}

func (t *amiOutput) getMime() string {
	return "application/x-qemu-disk"
}
