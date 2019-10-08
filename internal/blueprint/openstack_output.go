package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type openstackOutput struct{}

func (t *openstackOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := getF30Pipeline()
	addF30FSTabStage(p)
	addF30GRUB2Stage(p)
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "qcow2", t.getName())
	return p
}

func (t *openstackOutput) getName() string {
	return "image.qcow2"
}

func (t *openstackOutput) getMime() string {
	return "application/x-qemu-disk"
}
