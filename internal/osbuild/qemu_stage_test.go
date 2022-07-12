package osbuild

import (
	"fmt"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewQemuStage(t *testing.T) {

	formatOptionsList := []QEMUFormatOptions{
		QCOW2Options{
			Type:   QEMUFormatQCOW2,
			Compat: "0.10",
		},
		VDIOptions{
			Type: QEMUFormatVDI,
		},
		VPCOptions{
			Type: QEMUFormatVPC,
		},
		VMDKOptions{
			Type: QEMUFormatVMDK,
		},
		VHDXOptions{
			Type: QEMUFormatVHDX,
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

func TestNewQEMUStageOptions(t *testing.T) {
	tests := []struct {
		Filename        string
		Format          QEMUFormat
		FormatOptions   QEMUFormatOptions
		ExpectedOptions *QEMUStageOptions
		Error           bool
	}{
		{
			Filename: "image.qcow2",
			Format:   QEMUFormatQCOW2,
			FormatOptions: QCOW2Options{
				Compat: "1.1",
			},
			ExpectedOptions: &QEMUStageOptions{
				Filename: "image.qcow2",
				Format: QCOW2Options{
					Type:   QEMUFormatQCOW2,
					Compat: "1.1",
				},
			},
		},
		{
			Filename: "image.qcow2",
			Format:   QEMUFormatQCOW2,
			FormatOptions: QCOW2Options{
				Type: QEMUFormatQCOW2,
			},
			ExpectedOptions: &QEMUStageOptions{
				Filename: "image.qcow2",
				Format: QCOW2Options{
					Type: QEMUFormatQCOW2,
				},
			},
		},
		{
			Filename:      "image.qcow2",
			Format:        QEMUFormatQCOW2,
			FormatOptions: QCOW2Options{},
			ExpectedOptions: &QEMUStageOptions{
				Filename: "image.qcow2",
				Format: QCOW2Options{
					Type: QEMUFormatQCOW2,
				},
			},
		},
		{
			Filename:      "image.qcow2",
			Format:        QEMUFormatQCOW2,
			FormatOptions: nil,
			ExpectedOptions: &QEMUStageOptions{
				Filename: "image.qcow2",
				Format: QCOW2Options{
					Type: QEMUFormatQCOW2,
				},
			},
		},
		{
			Filename:      "image.vdi",
			Format:        QEMUFormatVDI,
			FormatOptions: nil,
			ExpectedOptions: &QEMUStageOptions{
				Filename: "image.vdi",
				Format: VDIOptions{
					Type: QEMUFormatVDI,
				},
			},
		},
		{
			Filename:      "image.vpc",
			Format:        QEMUFormatVPC,
			FormatOptions: nil,
			ExpectedOptions: &QEMUStageOptions{
				Filename: "image.vpc",
				Format: VPCOptions{
					Type: QEMUFormatVPC,
				},
			},
		},
		{
			Filename: "image.vpc",
			Format:   QEMUFormatVPC,
			FormatOptions: VPCOptions{
				ForceSize: common.BoolToPtr(false),
			},
			ExpectedOptions: &QEMUStageOptions{
				Filename: "image.vpc",
				Format: VPCOptions{
					Type:      QEMUFormatVPC,
					ForceSize: common.BoolToPtr(false),
				},
			},
		},
		{
			Filename:      "image.vmdk",
			Format:        QEMUFormatVMDK,
			FormatOptions: nil,
			ExpectedOptions: &QEMUStageOptions{
				Filename: "image.vmdk",
				Format: VMDKOptions{
					Type: QEMUFormatVMDK,
				},
			},
		},
		{
			Filename: "image.vmdk",
			Format:   QEMUFormatVMDK,
			FormatOptions: VMDKOptions{
				Subformat: VMDKSubformatStreamOptimized,
			},
			ExpectedOptions: &QEMUStageOptions{
				Filename: "image.vmdk",
				Format: VMDKOptions{
					Type:      QEMUFormatVMDK,
					Subformat: VMDKSubformatStreamOptimized,
				},
			},
		},
		{
			Filename:      "image.vhdx",
			Format:        QEMUFormatVHDX,
			FormatOptions: nil,
			ExpectedOptions: &QEMUStageOptions{
				Filename: "image.vhdx",
				Format: VHDXOptions{
					Type: QEMUFormatVHDX,
				},
			},
		},
		// mismatch between format and format options type
		{
			Filename:      "image.qcow2",
			Format:        QEMUFormatQCOW2,
			FormatOptions: VMDKOptions{},
			Error:         true,
		},
		// mismatch between format and format options type
		{
			Filename: "image.qcow2",
			Format:   QEMUFormatQCOW2,
			FormatOptions: VMDKOptions{
				Type: QEMUFormatQCOW2,
			},
			Error: true,
		},
		// mismatch between format and format options type
		{
			Filename: "image.qcow2",
			Format:   QEMUFormatQCOW2,
			FormatOptions: VMDKOptions{
				Type: QEMUFormatVMDK,
			},
			Error: true,
		},
		// unknown format
		{
			Filename:      "image.qcow2",
			Format:        "",
			FormatOptions: nil,
			Error:         true,
		},
	}
	for idx, test := range tests {
		t.Run(fmt.Sprintf("test-(%d/%d)", idx, len(tests)), func(t *testing.T) {
			if test.Error {
				assert.Panics(t, func() { NewQEMUStageOptions(test.Filename, test.Format, test.FormatOptions) })
			} else {
				stageOptions := NewQEMUStageOptions(test.Filename, test.Format, test.FormatOptions)
				assert.EqualValues(t, test.ExpectedOptions, stageOptions)
			}
		})
	}
}
