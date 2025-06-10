package osbuild

import (
	"fmt"
)

type VagrantProvider string

const (
	VagrantProviderLibvirt VagrantProvider = "libvirt"
)

type VagrantStageOptions struct {
	Provider VagrantProvider `json:"provider"`
}

func (VagrantStageOptions) isStageOptions() {}

type VagrantStageInputs struct {
	Image *FilesInput `json:"image"`
}

func (VagrantStageInputs) isStageInputs() {}

func NewVagrantStage(options *VagrantStageOptions, inputs *VagrantStageInputs) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.vagrant",
		Options: options,
		Inputs:  inputs,
	}
}

func NewVagrantStageOptions(provider VagrantProvider) *VagrantStageOptions {
	return &VagrantStageOptions{
		Provider: provider,
	}
}

func (o *VagrantStageOptions) validate() error {
	if o.Provider != VagrantProviderLibvirt {
		return fmt.Errorf("unknown provider in vagrant stage options %s", o.Provider)
	}

	return nil
}

func NewVagrantStagePipelineFilesInputs(pipeline, file string) *VagrantStageInputs {
	input := NewFilesInput(NewFilesInputPipelineObjectRef(pipeline, file, nil))
	return &VagrantStageInputs{Image: input}
}
