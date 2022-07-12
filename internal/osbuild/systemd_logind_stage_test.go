package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSystemdLogindStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.systemd-logind",
		Options: &SystemdLogindStageOptions{},
	}
	actualStage := NewSystemdLogindStage(&SystemdLogindStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestSystemdLogindStage_MarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		options SystemdLogindStageOptions
	}{
		{
			name:    "empty-options",
			options: SystemdLogindStageOptions{},
		},
		{
			name: "no-section-options",
			options: SystemdLogindStageOptions{
				Filename: "10-ec2-getty-fix.conf",
				Config: SystemdLogindConfigDropin{
					Login: SystemdLogindConfigLoginSection{},
				},
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
