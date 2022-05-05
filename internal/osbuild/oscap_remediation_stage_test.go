package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOscapRemediationStage(t *testing.T) {
	stageOptions := &OscapRemediationStageOptions{DataDir: "/var/tmp", Config: OscapConfig{
		Datastream: "test_stream",
		ProfileID:  "test_profile",
	}}
	expectedStage := &Stage{
		Type:    "org.osbuild.oscap.remediation",
		Options: stageOptions,
	}
	actualStage := NewOscapRemediationStage(stageOptions)
	assert.Equal(t, expectedStage, actualStage)
}

func TestOscapRemediationStageOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		options OscapRemediationStageOptions
		err     bool
	}{
		{
			name:    "empty-options",
			options: OscapRemediationStageOptions{},
			err:     true,
		},
		{
			name: "empty-datastream",
			options: OscapRemediationStageOptions{
				Config: OscapConfig{
					ProfileID: "test-profile",
				},
			},
			err: true,
		},
		{
			name: "empty-profile-id",
			options: OscapRemediationStageOptions{
				Config: OscapConfig{
					Datastream: "test-datastream",
				},
			},
			err: true,
		},
		{
			name: "invalid-verbosity-level",
			options: OscapRemediationStageOptions{
				Config: OscapConfig{
					Datastream:   "test-datastream",
					ProfileID:    "test-profile",
					VerboseLevel: "FAKE",
				},
			},
			err: true,
		},
		{
			name: "valid-data",
			options: OscapRemediationStageOptions{
				Config: OscapConfig{
					Datastream:   "test-datastream",
					ProfileID:    "test-profile",
					VerboseLevel: "INFO",
				},
			},
			err: false,
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err {
				assert.Errorf(t, tt.options.Config.validate(), "%q didn't return an error [idx: %d]", tt.name, idx)
				assert.Panics(t, func() { NewOscapRemediationStage(&tt.options) })
			} else {
				assert.NoErrorf(t, tt.options.Config.validate(), "%q returned an error [idx: %d]", tt.name, idx)
				assert.NotPanics(t, func() { NewOscapRemediationStage(&tt.options) })
			}
		})
	}
}
