package blueprint

import "osbuild-composer/internal/pipeline"

type diskOutput struct{}

func (t *diskOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := &pipeline.Pipeline{}
	p.SetAssembler(
		pipeline.NewQEMUAssembler(
			&pipeline.QEMUAssemblerOptions{
				Format:   "raw",
				Filename: t.getName(),
			}))
	return p
}

func (t *diskOutput) getName() string {
	return "image.img"
}

func (t *diskOutput) getMime() string {
	return "application/octet-stream"
}
