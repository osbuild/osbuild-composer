package blueprint

import "github.com/osbuild/osbuild-composer/internal/pipeline"

type vhdOutput struct{}

func (t *vhdOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := &pipeline.Pipeline{}
	p.SetAssembler(
		pipeline.NewQEMUAssembler(
			&pipeline.QEMUAssemblerOptions{
				Format:   "qcow2",
				Filename: t.getName(),
			}))
	return p
}

func (t *vhdOutput) getName() string {
	return "image.vhd"
}

func (t *vhdOutput) getMime() string {
	return "application/x-vhd"
}
