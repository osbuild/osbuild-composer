package fedora30

import (
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

type tarOutput struct{}

func (t *tarOutput) translate(b *blueprint.Blueprint) (*pipeline.Pipeline, error) {
	packages := [...]string{
		"policycoreutils",
		"selinux-policy-targeted",
		"kernel",
		"firewalld",
		"chrony",
		"langpacks-en",
	}
	excludedPackages := [...]string{
		"dracut-config-rescue",
	}
	p := newF30Pipeline(packages[:], excludedPackages[:], b)
	err := customizeAll(p, b.Customizations)
	if err != nil {
		return nil, err
	}
	setBootloader(p, "ro biosdevname=0 net.ifnames=0", b)
	setFirewall(p, nil, nil, b)
	setServices(p, nil, nil, b)
	setTarAssembler(p, t.getName(), "xz")

	return p, nil
}

func (t *tarOutput) getName() string {
	return "root.tar.xz"
}

func (t *tarOutput) getMime() string {
	return "application/x-tar"
}
