package blueprint

import "osbuild-composer/internal/pipeline"

type tarOutput struct{}

func (t *tarOutput) translate(b *Blueprint) *pipeline.Pipeline {
	p := getF30Pipeline()
	addF30SELinuxStage(p)
	addF30TarAssembler(p, t.getName())
	return p
}

func (t *tarOutput) getName() string {
	return "image.tar"
}

func (t *tarOutput) getMime() string {
	return "application/x-tar"
}
