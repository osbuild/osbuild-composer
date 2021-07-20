package osbuild2

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
	var mounts []Mount
	devices["root"] = Device{
		Type: "org.osbuild.loopback",
		Options: LoopbackDeviceOptions{
			Filename: "/somekindofimage.img",
			Start:    0,
			Size:     1073741824,
		},
	}
	treeInput := CopyStageInput{}
	treeInput.Type = "org.osbuild.tree"
	treeInput.Origin = "org.osbuild.pipeline"
	treeInput.References = []string{"name:input-pipeline"}
	copyStageMounts := CopyStageMounts(mounts)
	copyStageDevices := CopyStageDevices(devices)
	expectedStage := &Stage{
		Type:    "org.osbuild.copy",
		Options: &CopyStageOptions{paths},
		Inputs:  &CopyStageInputs{"tree-input": treeInput},
		Devices: &copyStageDevices,
		Mounts:  copyStageMounts,
	}
	actualStage := NewCopyStage(&CopyStageOptions{paths}, &CopyStageInputs{"tree-input": treeInput}, &copyStageDevices, copyStageMounts)
	assert.Equal(t, expectedStage, actualStage)
}
