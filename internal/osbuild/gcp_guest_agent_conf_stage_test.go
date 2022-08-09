package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGcpGuestAgentConfigOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		options GcpGuestAgentConfigOptions
		err     bool
	}{
		{
			name:    "empty-config",
			options: GcpGuestAgentConfigOptions{},
			err:     true,
		},
		{
			name: "empty-config",
			options: GcpGuestAgentConfigOptions{
				ConfigScope: GcpGuestAgentConfigScopeDistro,
				Config:      &GcpGuestAgentConfig{},
			},
			err: true,
		},
		{
			name: "invalid-ConfigScope",
			options: GcpGuestAgentConfigOptions{
				ConfigScope: "incorrect",
				Config: &GcpGuestAgentConfig{
					Accounts: &GcpGuestAgentConfigAccounts{
						Groups: []string{"group1", "group2"},
					},
				},
			},
			err: true,
		},
		{
			name: "valid-data",
			options: GcpGuestAgentConfigOptions{
				ConfigScope: GcpGuestAgentConfigScopeDistro,
				Config: &GcpGuestAgentConfig{
					Accounts: &GcpGuestAgentConfigAccounts{
						Groups: []string{"group1", "group2"},
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
				assert.Panics(t, func() { NewGcpGuestAgentConfigStage(&tt.options) })
			} else {
				assert.NoErrorf(t, tt.options.validate(), "%q returned an error [idx: %d]", tt.name, idx)
				assert.NotPanics(t, func() { NewGcpGuestAgentConfigStage(&tt.options) })
			}
		})
	}
}
func TestNewGcpGuestAgentConfigStage(t *testing.T) {
	expectedStage := &Stage{
		Type: "org.osbuild.gcp.guest-agent.conf",
		Options: &GcpGuestAgentConfigOptions{
			Config: &GcpGuestAgentConfig{
				Accounts: &GcpGuestAgentConfigAccounts{
					Groups: []string{"group1", "group2"},
				},
			},
		},
	}
	actualStage := NewGcpGuestAgentConfigStage(&GcpGuestAgentConfigOptions{
		Config: &GcpGuestAgentConfig{
			Accounts: &GcpGuestAgentConfigAccounts{
				Groups: []string{"group1", "group2"},
			},
		},
	})
	assert.Equal(t, expectedStage, actualStage)
}
