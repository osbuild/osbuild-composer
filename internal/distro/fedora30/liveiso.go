package fedora30

import (
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

type liveIsoOutput struct{}

func (t *liveIsoOutput) translate(b *blueprint.Blueprint) (*pipeline.Pipeline, error) {
	// TODO!
	p := getF30Pipeline()
	addF30SELinuxStage(p)
	addF30QemuAssembler(p, "raw", t.getName())

	if b.Customizations != nil {
		err := customizeAll(p, b.Customizations)
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (t *liveIsoOutput) getName() string {
	return "image.iso"
}

func (t *liveIsoOutput) getMime() string {
	return "application/x-iso9660-image"
}
