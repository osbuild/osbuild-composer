package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFSTabStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.fstab",
		Options: &FSTabStageOptions{},
	}
	actualStage := NewFSTabStage(&FSTabStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestAddFilesystem(t *testing.T) {
	options := &FSTabStageOptions{}
	filesystems := []*FSTabEntry{
		{
			UUID:    "76a22bf4-f153-4541-b6c7-0332c0dfaeac",
			VFSType: "ext4",
			Path:    "/",
			Options: "defaults",
			Freq:    1,
			PassNo:  1,
		},
		{
			UUID:    "bba22bf4-f153-4541-b6c7-0332c0dfaeac",
			VFSType: "xfs",
			Path:    "/home",
			Options: "defaults",
			Freq:    1,
			PassNo:  2,
		},
		{
			UUID:    "cca22bf4-f153-4541-b6c7-0332c0dfaeac",
			VFSType: "xfs",
			Path:    "/var",
			Options: "defaults",
			Freq:    1,
			PassNo:  1,
		},
	}

	for i, fs := range filesystems {
		options.AddFilesystem(fs.UUID, fs.VFSType, fs.Path, fs.Options, fs.Freq, fs.PassNo)
		assert.Equal(t, options.FileSystems[i], fs)
	}
	assert.Equal(t, len(filesystems), len(options.FileSystems))
}
