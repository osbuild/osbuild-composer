package osbuild

import (
	"fmt"
)

type FlatpakBuildImportOCIStageOptions struct {
	Repository string `json:"repository"`
}

func (FlatpakBuildImportOCIStageOptions) isStageOptions() {}

type FlatpakBuildImportOCIStageInputs struct {
	Containers ContainersInput `json:"containers"`
}

func (FlatpakBuildImportOCIStageInputs) isStageInputs() {}

func (inputs FlatpakBuildImportOCIStageInputs) validate() error {
	if inputs.Containers.References == nil {
		return fmt.Errorf("stage requires exactly 1 input container (got nil References)")
	}
	if ncontainers := len(inputs.Containers.References); ncontainers != 1 {
		return fmt.Errorf("stage requires exactly 1 input container (got %d)", ncontainers)
	}
	return nil
}

type FlatpakBuildImportOCIStageReference struct {
	Ref string `json:"ref"`
}

type FlatpakBuildImportOCIStageReferences map[string]FlatpakBuildImportOCIStageReference

func (FlatpakBuildImportOCIStageReferences) isReferences() {}

func NewFlatpakBuildImportOCIStage(options *FlatpakBuildImportOCIStageOptions, inputs *FlatpakBuildImportOCIStageInputs) (*Stage, error) {
	if err := inputs.validate(); err != nil {
		return nil, err
	}
	return &Stage{
		Type:    "org.osbuild.flatpak.build-import-bundle.oci",
		Inputs:  inputs,
		Options: options,
	}, nil
}
