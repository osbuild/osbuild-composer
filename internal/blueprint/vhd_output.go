package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type vhdOutput struct{}

func (t *vhdOutput) translate(b *Blueprint) (*pipeline.Pipeline, error) {
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
	excludedPackages := [...]string{
		"dracut-config-rescue",
	}
	p := getCustomF30PackageSet(packages[:], excludedPackages[:], b)
	addF30LocaleStage(p)
	addF30FSTabStage(p)
	addF30GRUB2Stage(p, b.getKernelCustomization())
	addF30FixBlsStage(p)
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "qcow2", t.getName())

	if b.Customizations != nil {
		err := b.Customizations.customizeAll(p)
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (t *vhdOutput) getName() string {
	return "image.vhd"
}

func (t *vhdOutput) getMime() string {
	return "application/x-vhd"
}
