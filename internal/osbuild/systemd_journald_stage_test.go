package osbuild

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestNewSystemdJournalStage(t *testing.T) {
	options := &SystemdJournaldStageOptions{
		Filename: "journald-config.conf",
		Config: SystemdJournaldConfigDropin{
			Journal: SystemdJournaldConfigJournalSection{
				Storage:    common.StringToPtr("persistent"),
				Compress:   common.StringToPtr("yes"),
				MaxFileSec: common.StringToPtr("10day"),
				Audit:      common.StringToPtr("yes"),
			},
		}}
	expectedStage := &Stage{
		Type:    "org.osbuild.systemd-journald",
		Options: options,
	}
	actualStage := NewSystemdJournaldStage(options)
	assert.Equal(t, expectedStage, actualStage)
}

func TestSystemdJournaldStage_ValidateInvalid(t *testing.T) {
	tests := []struct {
		name    string
		options SystemdJournaldStageOptions
	}{
		{
			name:    "empty-options",
			options: SystemdJournaldStageOptions{},
		},
		{
			name: "no-journal-section-options",
			options: SystemdJournaldStageOptions{
				Filename: "10-some-file.conf",
				Config: SystemdJournaldConfigDropin{
					Journal: SystemdJournaldConfigJournalSection{},
				},
			},
		},
	}
	for idx, te := range tests {
		t.Run(te.name, func(t *testing.T) {
			assert.Errorf(t, te.options.validate(), "%q didn't return an error [idx: %d]", te.name, idx)
			assert.Panics(t, func() { NewSystemdJournaldStage(&te.options) })
		})
	}
}
