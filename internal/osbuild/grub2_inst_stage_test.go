package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewGrub2InstStage(t *testing.T) {
	options := Grub2InstStageOptions{
		Filename: "img.raw",
		Platform: "i386-pc",
		Location: 2048,
		Core: CoreMkImage{
			Type:       "mkimage",
			PartLabel:  "gpt",
			Filesystem: "ext4",
		},
		Prefix: PrefixPartition{
			Type:      "partition",
			PartLabel: "gpt",
			Number:    1,
			Path:      "/boot/grub2",
		},
		SectorSize: common.Uint64ToPtr(512),
	}

	expectedStage := &Stage{
		Type:    "org.osbuild.grub2.inst",
		Options: &options,
	}

	actualStage := NewGrub2InstStage(&options)
	assert.Equal(t, expectedStage, actualStage)
}

func TestMarshalGrub2InstStage(t *testing.T) {
	goodOptions := func() Grub2InstStageOptions {
		return Grub2InstStageOptions{
			Filename: "img.raw",
			Platform: "i386-pc",
			Location: 2048,
			Core: CoreMkImage{
				Type:       "mkimage",
				PartLabel:  "gpt",
				Filesystem: "ext4",
			},
			Prefix: PrefixPartition{
				Type:      "partition",
				PartLabel: "gpt",
				Number:    1,
				Path:      "/boot/grub2",
			},
			SectorSize: common.Uint64ToPtr(512),
		}
	}

	{
		options := goodOptions()

		stage := NewGrub2InstStage(&options)
		_, err := json.Marshal(stage)
		assert.NoError(t, err)
	}

	{
		options := goodOptions()
		options.Core.Type = "notmkimage"

		stage := NewGrub2InstStage(&options)
		_, err := json.Marshal(stage)
		assert.Error(t, err)
	}

	{
		options := goodOptions()
		options.Core.PartLabel = "notgpt"

		stage := NewGrub2InstStage(&options)
		_, err := json.Marshal(stage)
		assert.Error(t, err)
	}

	{
		options := goodOptions()
		options.Core.Filesystem = "apfs"

		stage := NewGrub2InstStage(&options)
		_, err := json.Marshal(stage)
		assert.Error(t, err)
	}

	{
		options := goodOptions()
		options.Prefix.Type = "notpartition"

		stage := NewGrub2InstStage(&options)
		_, err := json.Marshal(stage)
		assert.Error(t, err)
	}

	{
		options := goodOptions()
		options.Prefix.PartLabel = "notdos"

		stage := NewGrub2InstStage(&options)
		_, err := json.Marshal(stage)
		assert.Error(t, err)
	}
}
