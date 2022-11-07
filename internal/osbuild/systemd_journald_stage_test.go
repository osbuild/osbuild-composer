package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSystemdJournalStage(t *testing.T) {
	options := &SystemdJournaldStageOptions{
		Filename: "journald-config.conf",
		Config: SystemdJournaldConfigDropin{
			Journal: SystemdJournaldConfigJournalSection{
				Storage:    StoragePresistent,
				Compress:   "yes",
				MaxFileSec: "10day",
				Audit:      AuditYes,
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

func TestInvalidFilename(t *testing.T) {
	options := &SystemdJournaldStageOptions{
		Filename: "invalid-filename",
		Config: SystemdJournaldConfigDropin{
			Journal: SystemdJournaldConfigJournalSection{
				Storage:    StoragePresistent,
				Compress:   "yes",
				MaxFileSec: "10day",
				Audit:      AuditYes,
			},
		},
	}

	assert.Errorf(t, options.validate(), "test didn't return any error ")
	assert.Panics(t, func() { NewSystemdJournaldStage(options) })
}
