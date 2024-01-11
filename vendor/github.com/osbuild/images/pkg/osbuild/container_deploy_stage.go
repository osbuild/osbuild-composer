package osbuild

import "fmt"

type ContainerDeployInputs struct {
	Images ContainersInput `json:"images"`
}

func (ContainerDeployInputs) isStageInputs() {}

func (inputs ContainerDeployInputs) validate() error {
	if inputs.Images.References == nil {
		return fmt.Errorf("stage requires exactly 1 input container (got nil References)")
	}
	if ncontainers := len(inputs.Images.References); ncontainers != 1 {
		return fmt.Errorf("stage requires exactly 1 input container (got %d)", ncontainers)
	}
	return nil
}

func NewContainerDeployStage(images ContainersInput) (*Stage, error) {
	inputs := ContainerDeployInputs{
		Images: images,
	}
	if err := inputs.validate(); err != nil {
		return nil, err
	}
	return &Stage{
		Type:   "org.osbuild.container-deploy",
		Inputs: inputs,
	}, nil
}
