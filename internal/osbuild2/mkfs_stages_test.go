package osbuild2

import (
	"testing"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewMkfsStage(t *testing.T) {
	devOpts := LoopbackDeviceOptions{
		Filename:   "file.img",
		Start:      0,
		Size:       1024,
		SectorSize: common.Uint64ToPtr(512),
	}
	device := NewLoopbackDevice(&devOpts)

	btrfsOptions := &MkfsBtrfsStageOptions{
		UUID:  uuid.New().String(),
		Label: "test",
	}
	btrfsDevices := &MkfsBtrfsStageDevices{Device: *device}
	mkbtrfs := NewMkfsBtrfsStage(btrfsOptions, btrfsDevices)
	mkbtrfsExpected := &Stage{
		Type:    "org.osbuild.mkfs.btrfs",
		Options: btrfsOptions,
		Devices: btrfsDevices,
	}
	assert.Equal(t, mkbtrfsExpected, mkbtrfs)

	ext4Options := &MkfsExt4StageOptions{
		UUID:  uuid.New().String(),
		Label: "test",
	}
	ext4Devices := &MkfsExt4StageDevices{Device: *device}
	mkext4 := NewMkfsExt4Stage(ext4Options, ext4Devices)
	mkext4Expected := &Stage{
		Type:    "org.osbuild.mkfs.ext4",
		Options: ext4Options,
		Devices: ext4Devices,
	}
	assert.Equal(t, mkext4Expected, mkext4)

	fatOptions := &MkfsFATStageOptions{
		VolID:   "7B7795E7",
		Label:   "test",
		FATSize: common.IntToPtr(12),
	}
	fatDevices := &MkfsFATStageDevices{Device: *device}
	mkfat := NewMkfsFATStage(fatOptions, fatDevices)
	mkfatExpected := &Stage{
		Type:    "org.osbuild.mkfs.fat",
		Options: fatOptions,
		Devices: fatDevices,
	}
	assert.Equal(t, mkfatExpected, mkfat)

	xfsOptions := &MkfsXfsStageOptions{
		UUID:  uuid.New().String(),
		Label: "test",
	}
	xfsDevices := &MkfsXfsStageDevices{Device: *device}
	mkxfs := NewMkfsXfsStage(xfsOptions, xfsDevices)
	mkxfsExpected := &Stage{
		Type:    "org.osbuild.mkfs.xfs",
		Options: xfsOptions,
		Devices: xfsDevices,
	}
	assert.Equal(t, mkxfsExpected, mkxfs)
}
