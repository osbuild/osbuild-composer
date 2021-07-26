package osbuild2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCloudInitStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.cloud-init",
		Options: &CloudInitStageOptions{},
	}
	actualStage := NewCloudInitStage(&CloudInitStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestCloudInitStage_MarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		options CloudInitStageOptions
	}{
		{
			name:    "empty-options",
			options: CloudInitStageOptions{},
		},
		{
			name: "no-config-file-section",
			options: CloudInitStageOptions{
				Filename: "00-default_user.cfg",
				Config:   CloudInitConfigFile{},
			},
		},
		{
			name: "no-system-info-section-option",
			options: CloudInitStageOptions{
				Filename: "00-default_user.cfg",
				Config: CloudInitConfigFile{
					SystemInfo: &CloudInitConfigSystemInfo{},
				},
			},
		},
		{
			name: "no-default-user-section-option",
			options: CloudInitStageOptions{
				Filename: "00-default_user.cfg",
				Config: CloudInitConfigFile{
					SystemInfo: &CloudInitConfigSystemInfo{
						DefaultUser: &CloudInitConfigDefaultUser{},
					},
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
