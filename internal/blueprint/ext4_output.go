package blueprint

import "osbuild-composer/internal/pipeline"

type ext4Output struct{}

func (t *ext4Output) translate(b *Blueprint) *pipeline.Pipeline {
	p := getF30Pipeline()
	addF30SELinuxStage(p)
	addF30RawFSAssembler(p, t.getName())
	return p
}

func (t *ext4Output) getName() string {
	return "image.img"
}

func (t *ext4Output) getMime() string {
	return "application/octet-stream"
}
