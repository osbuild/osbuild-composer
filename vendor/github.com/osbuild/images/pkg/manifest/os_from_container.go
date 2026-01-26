package manifest

import (
	"errors"
	"fmt"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/osbuild"
)

// OSFromContainer represents a pipeline that deploys an OS tree from a container.
type OSFromContainer struct {
	Base

	SourceContainer *container.SourceSpec

	sourceContainerSpec *container.Spec
}

var _ Pipeline = (*OSFromContainer)(nil)

func NewOSFromContainer(name string, build Build, srcContainer *container.SourceSpec) *OSFromContainer {
	p := &OSFromContainer{
		Base:            NewBase(name, build),
		SourceContainer: srcContainer,
	}
	build.addDependent(p)
	return p
}

func (p *OSFromContainer) getContainerSources() []container.SourceSpec {
	return []container.SourceSpec{*p.SourceContainer}
}

func (p *OSFromContainer) getContainerSpecs() []container.Spec {
	return []container.Spec{*p.sourceContainerSpec}
}

func (p *OSFromContainer) serializeStart(inputs Inputs) error {
	if p.sourceContainerSpec != nil {
		return errors.New("OSFromContainer: double call to serializeStart()")
	}
	if len(inputs.Containers) == 0 {
		return errors.New("OSFromContainer: no container in inputs")
	}
	if len(inputs.Containers) > 1 {
		return errors.New("OSFromContainer: multiple containers in inputs")
	}
	p.sourceContainerSpec = &inputs.Containers[0]
	return nil
}

func (p *OSFromContainer) serializeEnd() {
	p.sourceContainerSpec = nil
}

func (p *OSFromContainer) serialize() (osbuild.Pipeline, error) {
	if p.sourceContainerSpec == nil {
		return osbuild.Pipeline{}, fmt.Errorf("OSFromContainer: serialization not started")
	}

	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	image := osbuild.NewContainersInputForSingleSource(*p.sourceContainerSpec)
	stage, err := osbuild.NewContainerDeployStage(image, &osbuild.ContainerDeployOptions{RemoveSignatures: true})
	if err != nil {
		return pipeline, err
	}
	pipeline.AddStage(stage)

	return pipeline, nil
}
