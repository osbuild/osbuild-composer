package blueprint

import "osbuild-composer/internal/pipeline"

type tarOutput struct{}

func (t *tarOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := &pipeline.Pipeline{}
	p.SetAssembler(
		pipeline.NewTarAssembler(
			&pipeline.TarAssemblerOptions{
				Filename: "image.tar",
			}))
	return p
}

func (t *tarOutput) getName() string {
	return "image.tar"
}

func (t *tarOutput) getMime() string {
	return "application/x-tar"
}
