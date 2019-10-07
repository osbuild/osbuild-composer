package blueprint

import "osbuild-composer/internal/pipeline"

type liveIsoOutput struct{}

func (t *liveIsoOutput) translate(b *Blueprint) *pipeline.Pipeline {
	// TODO!
	p := getF30Pipeline()
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "raw", t.getName())
	return p
}

func (t *liveIsoOutput) getName() string {
	return "image.iso"
}

func (t *liveIsoOutput) getMime() string {
	return "application/x-iso9660-image"
}
