package osbuild2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewQemuStage(t *testing.T) {

	formatOptionsList := []QEMUFormatOptions{
		QCOW2Options{
			Type:   QEMUFormatQCOW2,
			Compat: "0.10",
		},
		VPCOptions{
			Type: QEMUFormatVPC,
		},
		VMDKOptions{
			Type: QEMUFormatVMDK,
		},
	}

	input := new(QEMUStageInput)
	input.Type = "org.osbuild.files"
	input.Origin = "org.osbuild.pipeline"
	input.References = map[string]QEMUFile{
		"name:stage": {
			File: "img.raw",
		},
	}
	inputs := QEMUStageInputs{Image: input}

	for _, format := range formatOptionsList {
		options := QEMUStageOptions{
			Filename: "img.out",
			Format:   format,
		}
		expectedStage := &Stage{
			Type:    "org.osbuild.qemu",
			Options: &options,
			Inputs:  &inputs,
		}

		actualStage := NewQEMUStage(&options, &inputs)
		assert.Equal(t, expectedStage, actualStage)
	}
}
