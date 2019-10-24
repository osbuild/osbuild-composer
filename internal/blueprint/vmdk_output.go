package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type vmdkOutput struct{}

func (t *vmdkOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := getF30Pipeline()
	addF30FSTabStage(p)
	addF30GRUB2Stage(p, b.getKernelCustomization())
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "vmdk", t.getName())
	return p
}

func (t *vmdkOutput) getName() string {
	return "image.vmdk"
}

func (t *vmdkOutput) getMime() string {
	return "application/x-vmdk"
}
