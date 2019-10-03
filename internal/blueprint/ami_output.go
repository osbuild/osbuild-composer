package blueprint

import "osbuild-composer/internal/pipeline"

type amiOutput struct{}

func (t *amiOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := &pipeline.Pipeline{}
	p.SetAssembler(
		pipeline.NewQEMUAssembler(
			&pipeline.QEMUAssemblerOptions{
				Format:   "qcow2",
				Filename: t.getName(),
			}))
	return p
}

func (t *amiOutput) getName() string {
	return "image.ami"
}

func (t *amiOutput) getMime() string {
	return "application/x-qemu-disk"
}
