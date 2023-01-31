package osbuild

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCopyStage(t *testing.T) {

	paths := []CopyStagePath{
		{
			From: "input://tree-input/",
			To:   "mount://root/",
		},
	}

	devices := make(map[string]Device)
	devices["root"] = Device{
		Type: "org.osbuild.loopback",
		Options: LoopbackDeviceOptions{
			Filename: "/somekindofimage.img",
			Start:    0,
			Size:     1073741824,
		},
	}

	mounts := []Mount{
		*NewBtrfsMount("root", "root", "/"),
	}

	treeInput := NewTreeInput("name:input-pipeline")
	expectedStage := &Stage{
		Type:    "org.osbuild.copy",
		Options: &CopyStageOptions{paths},
		Inputs:  &PipelineTreeInputs{"tree-input": *treeInput},
		Devices: devices,
		Mounts:  mounts,
	}
	// convert to alias types
	stageMounts := Mounts(mounts)
	stageDevices := Devices(devices)
	actualStage := NewCopyStage(&CopyStageOptions{paths}, NewPipelineTreeInputs("tree-input", "input-pipeline"), &stageDevices, &stageMounts)
	assert.Equal(t, expectedStage, actualStage)
}

func TestNewCopyStageSimpleSourcesInputs(t *testing.T) {
	fileSum := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	paths := []CopyStagePath{
		{
			From: fmt.Sprintf("input://inlinefile/sha256:%x", fileSum),
			To:   "tree://etc/inlinefile",
		},
	}

	filesInputs := CopyStageFilesInputs{
		"inlinefile": NewFilesInput(NewFilesInputSourceArrayRef([]FilesInputSourceArrayRefEntry{
			NewFilesInputSourceArrayRefEntry(fileSum, nil),
		})),
	}

	expectedStage := &Stage{
		Type:    "org.osbuild.copy",
		Options: &CopyStageOptions{paths},
		Inputs:  &filesInputs,
	}
	actualStage := NewCopyStageSimple(&CopyStageOptions{paths}, &filesInputs)
	assert.Equal(t, expectedStage, actualStage)
}
