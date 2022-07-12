package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTmpfilesdStageOptions(t *testing.T) {
	filename := "example.conf"
	config := []TmpfilesdConfigLine{{
		Type: "d",
		Path: "/tmp/my-example-path",
	}}

	expectedOptions := &TmpfilesdStageOptions{
		Filename: filename,
		Config:   config,
	}
	actualOptions := NewTmpfilesdStageOptions(filename, config)
	assert.Equal(t, expectedOptions, actualOptions)
}

func TestNewTmpfilesdStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.tmpfilesd",
		Options: &TmpfilesdStageOptions{},
	}
	actualStage := NewTmpfilesdStage(&TmpfilesdStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestTmpfilesdStageOptions_MarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		options TmpfilesdStageOptions
	}{
		{
			name:    "empty-options",
			options: TmpfilesdStageOptions{},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := json.Marshal(tt.options)
			assert.NotNilf(t, err, "json.Marshal() didn't return an error, but: %s [idx: %d]", string(gotBytes), idx)
		})
	}
}
