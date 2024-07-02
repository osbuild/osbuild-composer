package osbuild

import (
	"fmt"

	"github.com/osbuild/images/pkg/customizations/oscap"
)

type OscapAutotailorStageOptions struct {
	Filepath string                `json:"filepath"`
	Config   OscapAutotailorConfig `json:"config"`
}

type OscapAutotailorConfig struct {
	TailoredProfileID string   `json:"new_profile"`
	Datastream        string   `json:"datastream"`
	ProfileID         string   `json:"profile_id"`
	Selected          []string `json:"selected,omitempty"`
	Unselected        []string `json:"unselected,omitempty"`
}

func (OscapAutotailorStageOptions) isStageOptions() {}

func (c OscapAutotailorConfig) validate() error {
	if c.Datastream == "" {
		return fmt.Errorf("'datastream' must be specified")
	}
	if c.ProfileID == "" {
		return fmt.Errorf("'profile_id' must be specified")
	}
	if c.TailoredProfileID == "" {
		return fmt.Errorf("'new_profile' must be specified")
	}
	return nil
}

func NewOscapAutotailorStage(options *OscapAutotailorStageOptions) *Stage {
	if err := options.Config.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.oscap.autotailor",
		Options: options,
	}
}

func NewOscapAutotailorStageOptions(options *oscap.TailoringConfig) *OscapAutotailorStageOptions {
	if options == nil {
		return nil
	}

	// TODO: don't panic! unfortunately this would involve quite
	// a big refactor and we still need to be a bit defensive here
	if options.RemediationConfig.TailoringPath == "" {
		panic(fmt.Errorf("The tailoring path for the OpenSCAP remediation config cannot be empty, this is a programming error"))
	}

	return &OscapAutotailorStageOptions{
		Filepath: options.RemediationConfig.TailoringPath,
		Config: OscapAutotailorConfig{
			TailoredProfileID: options.TailoredProfileID,
			Datastream:        options.RemediationConfig.Datastream,
			ProfileID:         options.RemediationConfig.ProfileID,
			Selected:          options.Selected,
			Unselected:        options.Unselected,
		},
	}
}
