package blueprint

import "osbuild-composer/internal/pipeline"

type openstackOutput struct{}

func (t *openstackOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := &pipeline.Pipeline{}
	p.SetAssembler(
		pipeline.NewQEMUAssembler(
			&pipeline.QEMUAssemblerOptions{
				Format:   "qcow2",
				Filename: t.getName(),
			}))
	return p
}

func (t *openstackOutput) getName() string {
	return "image.qcow2"
}

func (t *openstackOutput) getMime() string {
	return "application/x-qemu-disk"
}
