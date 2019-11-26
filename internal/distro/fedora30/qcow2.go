package fedora30

import (
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

type qcow2Output struct{}

func (t *qcow2Output) translate(b *blueprint.Blueprint) (*pipeline.Pipeline, error) {
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

func (t *qcow2Output) getName() string {
	return "image.qcow2"
}

func (t *qcow2Output) getMime() string {
	return "application/x-qemu-disk"
}
