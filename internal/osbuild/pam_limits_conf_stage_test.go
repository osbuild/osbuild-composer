package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPamLimitsConfStageOptions(t *testing.T) {
	filename := "example.conf"
	config := []PamLimitsConfigLine{{
		Domain: "user1",
		Type:   PamLimitsTypeHard,
		Item:   PamLimitsItemCpu,
		Value:  PamLimitsValueInt(123),
	}}

	expectedOptions := &PamLimitsConfStageOptions{
		Filename: filename,
		Config:   config,
	}
	actualOptions := NewPamLimitsConfStageOptions(filename, config)
	assert.Equal(t, expectedOptions, actualOptions)
}

func TestNewPamLimitsConfStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.pam.limits.conf",
		Options: &PamLimitsConfStageOptions{},
	}
	actualStage := NewPamLimitsConfStage(&PamLimitsConfStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestPamLimitsConfStageOptions_MarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		options PamLimitsConfStageOptions
	}{
		{
			name:    "empty-options",
			options: PamLimitsConfStageOptions{},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := json.Marshal(tt.options)
			assert.NotNilf(t, err, "json.Marshal() didn't return an error, but: %s [idx: %d]", string(gotBytes), idx)
		})
	}
}
