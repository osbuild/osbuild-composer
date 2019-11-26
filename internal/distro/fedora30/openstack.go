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
	p := newF30Pipeline(packages[:], excludedPackages[:], b)
	err := customizeAll(p, b.Customizations)
	if err != nil {
		return nil, err
	}
	setFilesystems(p)
	setBootloader(p, "ro biosdevname=0 net.ifnames=0", b)
	setFirewall(p, nil, nil, b)
	setServices(p, nil, nil, b)
	setQemuAssembler(p, "qcow2", t.getName())

	return p, nil
}

func (t *openstackOutput) getName() string {
	return "image.qcow2"
}

func (t *openstackOutput) getMime() string {
	return "application/x-qemu-disk"
}
