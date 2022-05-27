package osbuild2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSfdiskStage(t *testing.T) {

	partition := SfdiskPartition{
		Bootable: true,
		Name:     "root",
		Size:     2097152,
		Start:    0,
		Type:     "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
		UUID:     "68B2905B-DF3E-4FB3-80FA-49D1E773AA33",
	}

	options := SfdiskStageOptions{
		Label:      "gpt",
		UUID:       "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Partitions: []SfdiskPartition{partition},
	}

	device := NewLoopbackDevice(&LoopbackDeviceOptions{Filename: "disk.raw"})
	devices := Devices{"device": *device}

	expectedStage := &Stage{
		Type:    "org.osbuild.sfdisk",
		Options: &options,
		Devices: devices,
	}

	actualStage := NewSfdiskStage(&options, device)
	assert.Equal(t, expectedStage, actualStage)
}
