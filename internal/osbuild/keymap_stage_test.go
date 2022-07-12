package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewKeymapStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.keymap",
		Options: &KeymapStageOptions{},
	}
	actualStage := NewKeymapStage(&KeymapStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestKeymapStage_MarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		options KeymapStageOptions
	}{
		{
			name: "x11-keymap-empty-layout-list",
			options: KeymapStageOptions{
				X11Keymap: &X11KeymapOptions{},
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
