package blueprint

import "osbuild-composer/internal/pipeline"

type qcow2Output struct{}

func (t *qcow2Output) translate(b *Blueprint) *pipeline.Pipeline {
	p := &pipeline.Pipeline{}
	p.SetAssembler(
		pipeline.NewQEMUAssembler(
			&pipeline.QEMUAssemblerOptions{
				Format:   "qcow2",
				Filename: t.getName(),
			}))
	return p
}

func (t *qcow2Output) getName() string {
	return "image.qcow2"
}

func (t *qcow2Output) getMime() string {
	return "application/x-qemu-disk"
}
