package osbuild2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewModprobeStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.modprobe",
		Options: &ModprobeStageOptions{},
	}
	actualStage := NewModprobeStage(&ModprobeStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestModprobeStage_MarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		options ModprobeStageOptions
	}{
		{
			name:    "empty-options",
			options: ModprobeStageOptions{},
		},
		{
			name: "no-commands",
			options: ModprobeStageOptions{
				Filename: "disallow-modules.conf",
				Commands: ModprobeConfigCmdList{},
			},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := json.Marshal(tt.options)
			assert.NotNilf(t, err, "json.Marshal() didn't return an error, but: %s [idx: %d]", string(gotBytes), idx)
		})
	}
}
