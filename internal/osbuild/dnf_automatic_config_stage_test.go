package osbuild

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewDNFAutomaticConfigStage(t *testing.T) {
	stageOptions := NewDNFAutomaticConfigStageOptions(&DNFAutomaticConfig{})
	expectedStage := &Stage{
		Type:    "org.osbuild.dnf-automatic.config",
		Options: stageOptions,
	}
	actualStage := NewDNFAutomaticConfigStage(stageOptions)
	assert.Equal(t, expectedStage, actualStage)
}

func TestDNFAutomaticConfigStageOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		options DNFAutomaticConfigStageOptions
		err     bool
	}{
		{
			name:    "empty-options",
			options: DNFAutomaticConfigStageOptions{},
			err:     false,
		},
		{
			name: "invalid-upgrade_type",
			options: DNFAutomaticConfigStageOptions{
				Config: &DNFAutomaticConfig{
					Commands: &DNFAutomaticConfigCommands{
						ApplyUpdates: common.BoolToPtr(true),
						UpgradeType:  "invalid",
					},
				},
			},
			err: true,
		},
		{
			name: "valid-data-1",
			options: DNFAutomaticConfigStageOptions{
				Config: &DNFAutomaticConfig{
					Commands: &DNFAutomaticConfigCommands{
						ApplyUpdates: common.BoolToPtr(true),
						UpgradeType:  DNFAutomaticUpgradeTypeDefault,
					},
				},
			},
			err: false,
		},
		{
			name: "valid-data-2",
			options: DNFAutomaticConfigStageOptions{
				Config: &DNFAutomaticConfig{
					Commands: &DNFAutomaticConfigCommands{
						ApplyUpdates: common.BoolToPtr(false),
						UpgradeType:  DNFAutomaticUpgradeTypeSecurity,
					},
				},
			},
			err: false,
		},
	}
	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err {
				assert.Errorf(t, tt.options.validate(), "%q didn't return an error [idx: %d]", tt.name, idx)
				assert.Panics(t, func() { NewDNFAutomaticConfigStage(&tt.options) })
			} else {
				assert.NoErrorf(t, tt.options.validate(), "%q returned an error [idx: %d]", tt.name, idx)
				assert.NotPanics(t, func() { NewDNFAutomaticConfigStage(&tt.options) })
			}
		})
	}
}
