package osbuild2

import (
	"encoding/json"
	"fmt"
)

// Convert a disk image to a different format.
//
// Some formats support format-specific options:
//   qcow2: The compatibility version can be specified via 'compat'

type QEMUStageOptions struct {
	// Filename for resulting image
	Filename string `json:"filename"`

	// Image format and options
	Format QEMUFormatOptions `json:"format"`
}

func (QEMUStageOptions) isStageOptions() {}

type QEMUFormatOptions interface {
	isQEMUFormatOptions()
}

type Qcow2Options struct {
	// The type of the format must be 'qcow2'
	Type string `json:"type"`

	// The qcow2-compatibility-version to use
	Compat string `json:"compat"`
}

func (Qcow2Options) isQEMUFormatOptions() {}

type VPCOptions struct {
	// The type of the format must be 'vpc'
	Type string `json:"type"`
}

func (VPCOptions) isQEMUFormatOptions() {}

type VMDKOptions struct {
	// The type of the format must be 'vpc'
	Type string `json:"type"`
}

func (VMDKOptions) isQEMUFormatOptions() {}

type QEMUStageInputs struct {
	Image *QEMUStageInput `json:"image"`
}

func (QEMUStageInputs) isStageInputs() {}

type QEMUStageInput struct {
	inputCommon
	References QEMUStageReferences `json:"references"`
}

func (QEMUStageInput) isStageInput() {}

type QEMUStageReferences map[string]QEMUFile

func (QEMUStageReferences) isReferences() {}

type QEMUFile struct {
	Metadata FileMetadata `json:"metadata,omitempty"`
	File     string       `json:"file,omitempty"`
}

type FileMetadata map[string]interface{}

// NewQEMUStage creates a new QEMU Stage object.
func NewQEMUStage(options *QEMUStageOptions, inputs *QEMUStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.qemu",
		Options: options,
		Inputs:  inputs,
	}
}

// alias for custom marshaller
type qemuStageOptions QEMUStageOptions

// Custom marshaller for validating
func (options QEMUStageOptions) MarshalJSON() ([]byte, error) {
	switch o := options.Format.(type) {
	case Qcow2Options:
		if o.Type != "qcow2" {
			return nil, fmt.Errorf("invalid format type %q for qcow2 options", o.Type)
		}
	case VPCOptions:
		if o.Type != "vpc" {
			return nil, fmt.Errorf("invalid format type %q for vpc options", o.Type)
		}
	case VMDKOptions:
		if o.Type != "vmdk" {
			return nil, fmt.Errorf("invalid format type %q for vmdk options", o.Type)
		}
	default:
		return nil, fmt.Errorf("unknown format options in QEMU stage: %#v", options.Format)
	}

	return json.Marshal(qemuStageOptions(options))
}

func NewQemuStagePipelineFilesInputs(stage, file string) *QEMUStageInputs {
	stageKey := "name:" + stage
	ref := map[string]QEMUFile{
		stageKey: {
			File: file,
		},
	}
	input := new(QEMUStageInput)
	input.Type = "org.osbuild.files"
	input.Origin = "org.osbuild.pipeline"
	input.References = ref
	return &QEMUStageInputs{Image: input}
}
