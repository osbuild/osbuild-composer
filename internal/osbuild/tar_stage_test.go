package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTarStage(t *testing.T) {
	stageOptions := &TarStageOptions{Filename: "archive.tar.xz"}
	stageInputs := &TarStageInputs{}
	expectedStage := &Stage{
		Type:    "org.osbuild.tar",
		Options: stageOptions,
		Inputs:  stageInputs,
	}
	actualStage := NewTarStage(stageOptions, stageInputs)
	assert.Equal(t, expectedStage, actualStage)
}

func TestTarStageOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		options TarStageOptions
		err     bool
	}{
		{
			name:    "empty-options",
			options: TarStageOptions{},
			err:     false,
		},
		{
			name: "invalid-archive-format",
			options: TarStageOptions{
				Filename: "archive.tar.xz",
				Format:   "made-up-format",
			},
			err: true,
		},
		{
			name: "invalid-root-node",
			options: TarStageOptions{
				Filename: "archive.tar.xz",
				RootNode: "I-don't-care",
			},
			err: true,
		},
		{
			name: "valid-data",
			options: TarStageOptions{
				Filename: "archive.tar.xz",
				Format:   TarArchiveFormatOldgnu,
				RootNode: TarRootNodeOmit,
			},
			err: false,
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err {
				assert.Errorf(t, tt.options.validate(), "%q didn't return an error [idx: %d]", tt.name, idx)
				assert.Panics(t, func() { NewTarStage(&tt.options, &TarStageInputs{}) })
			} else {
				assert.NoErrorf(t, tt.options.validate(), "%q returned an error [idx: %d]", tt.name, idx)
				assert.NotPanics(t, func() { NewTarStage(&tt.options, &TarStageInputs{}) })
			}
		})
	}
}
