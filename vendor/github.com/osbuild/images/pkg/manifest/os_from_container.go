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

	// PayloadContainer is an optional container to embed in the image's
	// container storage (e.g., for bootc installer ISOs that need the
	// payload container available at install time).
	PayloadContainer *container.SourceSpec

	sourceContainerSpec  *container.Spec
	payloadContainerSpec *container.Spec
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
	sources := []container.SourceSpec{*p.SourceContainer}
	if p.PayloadContainer != nil {
		sources = append(sources, *p.PayloadContainer)
	}
	return sources
}

func (p *OSFromContainer) getContainerSpecs() []container.Spec {
	specs := []container.Spec{*p.sourceContainerSpec}
	if p.payloadContainerSpec != nil {
		specs = append(specs, *p.payloadContainerSpec)
	}
	return specs
}

func (p *OSFromContainer) serializeStart(inputs Inputs) error {
	if p.sourceContainerSpec != nil {
		return errors.New("OSFromContainer: double call to serializeStart()")
	}
	expectedContainers := 1
	if p.PayloadContainer != nil {
		expectedContainers = 2
	}
	if len(inputs.Containers) < 1 {
		return errors.New("OSFromContainer: no container in inputs")
	}
	if len(inputs.Containers) != expectedContainers {
		return fmt.Errorf("OSFromContainer: expected %d containers in inputs, got %d", expectedContainers, len(inputs.Containers))
	}
	// The first container is the source container for deployment
	p.sourceContainerSpec = &inputs.Containers[0]
	// The second container (if present) is the payload container to embed in storage
	if len(inputs.Containers) > 1 {
		p.payloadContainerSpec = &inputs.Containers[1]
	}
	return nil
}

func (p *OSFromContainer) serializeEnd() {
	p.sourceContainerSpec = nil
	p.payloadContainerSpec = nil
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

	// Embed payload container in container storage
	if p.payloadContainerSpec != nil {
		for _, stage := range osbuild.GenContainerStorageStages("", []container.Spec{*p.payloadContainerSpec}) {
			pipeline.AddStage(stage)
		}
	}

	return pipeline, nil
}
