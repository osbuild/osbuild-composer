package osbuild

import (
	"fmt"
)

type SystemdJournaldStageOptions struct {
	Filename string                      `json:"filename"`
	Config   SystemdJournaldConfigDropin `json:"config"`
}

func (SystemdJournaldStageOptions) isStageOptions() {}

func (o SystemdJournaldStageOptions) validate() error {
	if o.Config.Journal == (SystemdJournaldConfigJournalSection{}) {
		return fmt.Errorf("the 'Journal' section is required")
	}
	if o.Config.Journal.Storage == nil && o.Config.Journal.Compress == nil && o.Config.Journal.SplitMode == nil && o.Config.Journal.MaxFileSec == nil && o.Config.Journal.MaxRetentionSec == nil && o.Config.Journal.SyncIntervalSec == nil && o.Config.Journal.Audit == nil {
		return fmt.Errorf("at least one 'Journal' section must be specified")
	}
	return nil
}

func NewSystemdJournaldStage(options *SystemdJournaldStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}
	return &Stage{
		Type:    "org.osbuild.systemd-journald",
		Options: options,
	}
}

type SystemdJournaldConfigDropin struct {
	Journal SystemdJournaldConfigJournalSection `json:"Journal"`
}

// 'Journal' configuration section, at least one option must be specified
type SystemdJournaldConfigJournalSection struct {
	// Controls where to store journal data.
	Storage *string `json:"Storage,omitempty"`

	// Sets whether the data objects stored in the journal should be
	// compressed or not. Can also take threshold values.
	Compress *string `json:"Compress,omitempty"`

	// Splits journal files per user or to a single file.
	SplitMode *string `json:"SplitMode,omitempty"`

	// Max time to store entries in a single file. By default seconds, may be
	// sufixed with units (year, month, week, day, h, m) to override this.
	MaxFileSec *string `json:"MaxFileSec,omitempty"`

	// Maximum time to store journal entries. By default seconds, may be sufixed
	// with units (year, month, week, day, h, m) to override this.
	MaxRetentionSec *string `json:"MaxRetentionSec,omitempty"`

	// Timeout before synchronizing journal files to disk. Minimum 0.
	SyncIntervalSec *int `json:"SyncIntervalSec,omitempty"`

	// Enables/Disables kernel auditing on start-up, leaves it as is if
	// unspecified.
	Audit *string `json:"Audit,omitempty"`
}
