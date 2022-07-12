package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewChronyStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.chrony",
		Options: &ChronyStageOptions{},
	}
	actualStage := NewChronyStage(&ChronyStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestChronyStage_MarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		options ChronyStageOptions
	}{
		{
			name:    "not-timeservers-nor-servers",
			options: ChronyStageOptions{},
		},
		{
			name: "timeservers-and-servers",
			options: ChronyStageOptions{
				Timeservers: []string{"ntp.example.com"},
				Servers:     []ChronyConfigServer{{Hostname: "ntp2.example.com"}},
			},
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := json.Marshal(tt.options)
			assert.NotNilf(t, err, "json.Marshal() didn't return an error [idx: %d]", idx)
		})
	}
}
