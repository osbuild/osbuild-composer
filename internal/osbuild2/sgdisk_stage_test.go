package osbuild2

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewSgdiskStage(t *testing.T) {

	uid := uuid.MustParse("68B2905B-DF3E-4FB3-80FA-49D1E773AA33")
	partition := SgdiskPartition{
		Bootable: true,
		Name:     "root",
		Size:     2097152,
		Start:    0,
		Type:     "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
		UUID:     &uid,
	}

	options := SgdiskStageOptions{
		UUID:       uuid.MustParse("D209C89E-EA5E-4FBD-B161-B461CCE297E0"),
		Partitions: []SgdiskPartition{partition},
	}

	device := NewLoopbackDevice(&LoopbackDeviceOptions{Filename: "disk.raw"})
	devices := Devices{"device": *device}

	expectedStage := &Stage{
		Type:    "org.osbuild.sgdisk",
		Options: &options,
		Devices: devices,
	}

	actualStage := NewSgdiskStage(&options, device)
	assert.Equal(t, expectedStage, actualStage)
}
