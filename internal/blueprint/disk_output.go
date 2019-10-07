package blueprint

import "osbuild-composer/internal/pipeline"

type diskOutput struct{}

func (t *diskOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := getF30Pipeline()
	addF30FSTabStage(p)
	addF30GRUB2Stage(p)
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "raw", t.getName())
	return p
}

func (t *diskOutput) getName() string {
	return "image.img"
}

func (t *diskOutput) getMime() string {
	return "application/octet-stream"
}
