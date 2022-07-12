package osbuild

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTunedStageOptions(t *testing.T) {
	tests := []struct {
		profiles        []string
		expectedOptions *TunedStageOptions
	}{
		{
			profiles:        []string{"balanced"},
			expectedOptions: &TunedStageOptions{Profiles: []string{"balanced"}},
		},
		{
			profiles:        []string{"balanced", "sap-hana"},
			expectedOptions: &TunedStageOptions{Profiles: []string{"balanced", "sap-hana"}},
		},
	}

	for idx, tt := range tests {
		t.Run(fmt.Sprint(idx), func(t *testing.T) {
			actualOptions := NewTunedStageOptions(tt.profiles...)
			assert.Equalf(t, tt.expectedOptions, actualOptions, "NewTunedStageOptions() failed [idx: %d]", idx)
		})
	}
}

func TestNewTunedStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.tuned",
		Options: &TunedStageOptions{},
	}
	actualStage := NewTunedStage(&TunedStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestTunedStageOptions_MarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		options TunedStageOptions
	}{
		{
			name:    "empty-options",
			options: TunedStageOptions{},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := json.Marshal(tt.options)
			assert.NotNilf(t, err, "json.Marshal() didn't return an error, but: %s [idx: %d]", string(gotBytes), idx)
		})
	}
}
