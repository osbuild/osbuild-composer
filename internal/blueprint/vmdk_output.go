package blueprint

import "osbuild-composer/internal/pipeline"

type vmdkOutput struct{}

func (t *vmdkOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := &pipeline.Pipeline{}
	p.SetAssembler(
		pipeline.NewQEMUAssembler(
			&pipeline.QEMUAssemblerOptions{
				Format:   "vmdk",
				Filename: t.getName(),
			}))
	return p
}

func (t *vmdkOutput) getName() string {
	return "image.vmdk"
}

func (t *vmdkOutput) getMime() string {
	return "application/x-vmdk"
}
