package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type amiOutput struct{}

func (t *amiOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := getF30Pipeline()
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
