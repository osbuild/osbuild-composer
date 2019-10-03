package blueprint

import "osbuild-composer/internal/pipeline"

type ext4Output struct{}

func (t *ext4Output) translate(b *Blueprint) *pipeline.Pipeline {
	p := &pipeline.Pipeline{}
	p.SetAssembler(
		pipeline.NewQEMUAssembler(
			&pipeline.QEMUAssemblerOptions{
				Format:   "raw",
				Filename: t.getName(),
			}))
	return p
}

func (t *ext4Output) getName() string {
	return "image.img"
}

func (t *ext4Output) getMime() string {
	return "application/octet-stream"
}
