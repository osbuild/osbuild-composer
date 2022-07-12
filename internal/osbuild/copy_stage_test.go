package osbuild

import (
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

	treeInput := CopyStageInput{}
	treeInput.Type = "org.osbuild.tree"
	treeInput.Origin = "org.osbuild.pipeline"
	treeInput.References = []string{"name:input-pipeline"}
	expectedStage := &Stage{
		Type:    "org.osbuild.copy",
		Options: &CopyStageOptions{paths},
		Inputs:  &CopyStageInputs{"tree-input": treeInput},
		Devices: devices,
		Mounts:  mounts,
	}
	// convert to alias types
	stageMounts := Mounts(mounts)
	stageDevices := Devices(devices)
	actualStage := NewCopyStage(&CopyStageOptions{paths}, &CopyStageInputs{"tree-input": treeInput}, &stageDevices, &stageMounts)
	assert.Equal(t, expectedStage, actualStage)
}
