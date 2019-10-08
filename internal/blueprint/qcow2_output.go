package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type qcow2Output struct{}

func (t *qcow2Output) translate(b *Blueprint) *pipeline.Pipeline {
	p := getF30Pipeline()
	addF30FSTabStage(p)
	addF30GRUB2Stage(p)
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "qcow2", t.getName())
	return p
}

func (t *qcow2Output) getName() string {
	return "image.qcow2"
}

func (t *qcow2Output) getMime() string {
	return "application/x-qemu-disk"
}
