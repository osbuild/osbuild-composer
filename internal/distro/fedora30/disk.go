package fedora30

import (
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

type diskOutput struct{}

func (t *diskOutput) translate(b *blueprint.Blueprint) (*pipeline.Pipeline, error) {
	packages := [...]string{
		"@core",
		"chrony",
		"firewalld",
		"grub2-pc",
		"kernel",
		"langpacks-en",
		"selinux-policy-targeted",
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
	setQemuAssembler(p, "raw", t.getName())

	return p, nil
}

func (t *diskOutput) getName() string {
	return "disk.img"
}

func (t *diskOutput) getMime() string {
	return "application/octet-stream"
}
