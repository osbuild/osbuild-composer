package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type amiOutput struct{}

func (t *amiOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := &pipeline.Pipeline{
		BuildPipeline: getF30BuildPipeline(),
	}

	options := &pipeline.DNFStageOptions{
		ReleaseVersion:   "30",
		BaseArchitecture: "x86_64",
	}
	options.AddRepository(getF30Repository())
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
	for _, pkg := range packages {
		options.AddPackage(pkg)
	}
	p.AddStage(pipeline.NewDNFStage(options))
	addF30FixBlsStage(p)
	addF30LocaleStage(p)
	addF30FSTabStage(p)
	addF30GRUB2Stage(p)
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "qcow2", t.getName())
	return p
}

func (t *amiOutput) getName() string {
	return "image.ami"
}

func (t *amiOutput) getMime() string {
	return "application/x-qemu-disk"
}
