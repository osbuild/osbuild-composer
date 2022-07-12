package osbuild

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

	devices := map[string]Device{
		"device": *device,
	}

	btrfsOptions := &MkfsBtrfsStageOptions{
		UUID:  uuid.New().String(),
		Label: "test",
	}
	mkbtrfs := NewMkfsBtrfsStage(btrfsOptions, devices)
	mkbtrfsExpected := &Stage{
		Type:    "org.osbuild.mkfs.btrfs",
		Options: btrfsOptions,
		Devices: Devices{"device": *device},
	}
	assert.Equal(t, mkbtrfsExpected, mkbtrfs)

	ext4Options := &MkfsExt4StageOptions{
		UUID:  uuid.New().String(),
		Label: "test",
	}
	mkext4 := NewMkfsExt4Stage(ext4Options, devices)
	mkext4Expected := &Stage{
		Type:    "org.osbuild.mkfs.ext4",
		Options: ext4Options,
		Devices: Devices{"device": *device},
	}
	assert.Equal(t, mkext4Expected, mkext4)

	fatOptions := &MkfsFATStageOptions{
		VolID:   "7B7795E7",
		Label:   "test",
		FATSize: common.IntToPtr(12),
	}
	mkfat := NewMkfsFATStage(fatOptions, devices)
	mkfatExpected := &Stage{
		Type:    "org.osbuild.mkfs.fat",
		Options: fatOptions,
		Devices: Devices{"device": *device},
	}
	assert.Equal(t, mkfatExpected, mkfat)

	xfsOptions := &MkfsXfsStageOptions{
		UUID:  uuid.New().String(),
		Label: "test",
	}
	mkxfs := NewMkfsXfsStage(xfsOptions, devices)
	mkxfsExpected := &Stage{
		Type:    "org.osbuild.mkfs.xfs",
		Options: xfsOptions,
		Devices: Devices{"device": *device},
	}
	assert.Equal(t, mkxfsExpected, mkxfs)
}
