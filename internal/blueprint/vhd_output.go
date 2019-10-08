package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type vhdOutput struct{}

func (t *vhdOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := getF30Pipeline()
	addF30FSTabStage(p)
	addF30GRUB2Stage(p)
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "qcow2", t.getName())
	return p
}

func (t *vhdOutput) getName() string {
	return "image.vhd"
}

func (t *vhdOutput) getMime() string {
	return "application/x-vhd"
}
