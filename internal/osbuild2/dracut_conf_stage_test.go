package osbuild2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDracutConfStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.dracut.conf",
		Options: &DracutConfStageOptions{},
	}
	actualStage := NewDracutConfStageOptions(&DracutConfStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestDracutConfStage_MarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		options DracutConfStageOptions
	}{
		{
			name: "no-options-in-config",
			options: DracutConfStageOptions{
				ConfigFiles: map[string]DracutConfigFile{
					"testing.conf": {},
				},
			},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := json.Marshal(tt.options)
			assert.NotNilf(t, err, "json.Marshal() didn't return an error, but %s [idx: %d]", string(gotBytes), idx)
		})
	}
}
