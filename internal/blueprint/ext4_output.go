package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type ext4Output struct{}

func (t *ext4Output) translate(b *Blueprint) *pipeline.Pipeline {
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
	p := getCustomF30PackageSet(packages[:], excludedPackages[:])
	addF30LocaleStage(p)
	addF30GRUB2Stage(p, b.getKernelCustomization())
	addF30FixBlsStage(p)
	addF30SELinuxStage(p)
	addF30RawFSAssembler(p, t.getName())
	return p
}

func (t *ext4Output) getName() string {
	return "filesystem.img"
}

func (t *ext4Output) getMime() string {
	return "application/octet-stream"
}
