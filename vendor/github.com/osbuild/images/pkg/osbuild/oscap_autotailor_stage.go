package osbuild

import (
	"fmt"

	"github.com/osbuild/images/pkg/customizations/oscap"
)

type OscapAutotailorStageOptions struct {
	Filepath string                `json:"filepath"`
	Config   OscapAutotailorConfig `json:"config"`
}

type OscapAutotailorConfig interface {
	validate() error
	isAutotailorConfig()
}

type AutotailorKeyValueConfig struct {
	NewProfile string   `json:"new_profile"`
	Datastream string   `json:"datastream"`
	ProfileID  string   `json:"profile_id"`
	Selected   []string `json:"selected,omitempty"`
	Unselected []string `json:"unselected,omitempty"`
}

func (c AutotailorKeyValueConfig) isAutotailorConfig() {}

func (c AutotailorKeyValueConfig) validate() error {
	if c.Datastream == "" {
		return fmt.Errorf("'datastream' must be specified")
	}
	if c.NewProfile == "" {
		return fmt.Errorf("'new_profile' must be specified")
	}
	if c.ProfileID == "" {
		return fmt.Errorf("'profile_id' must be specified")
	}
	return nil
}

type AutotailorJSONConfig struct {
	TailoredProfileID string `json:"tailored_profile_id"`
	Datastream        string `json:"datastream"`
	TailoringFile     string `json:"tailoring_file"`
}

func (c AutotailorJSONConfig) isAutotailorConfig() {}

func (c AutotailorJSONConfig) validate() error {
	if c.Datastream == "" {
		return fmt.Errorf("'datastream' must be specified")
	}
	if c.TailoredProfileID == "" {
		return fmt.Errorf("'tailored_profile_id' must be specified")
	}
	if c.TailoringFile == "" {
		return fmt.Errorf("'tailoring_file' must be specified")
	}
	return nil
}

func (OscapAutotailorStageOptions) isStageOptions() {}

func NewOscapAutotailorStage(options *OscapAutotailorStageOptions) *Stage {
	if err := options.Config.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.oscap.autotailor",
		Options: options,
	}
}

func NewOscapAutotailorStageOptions(options *oscap.RemediationConfig) *OscapAutotailorStageOptions {
	if options == nil {
		return nil
	}

	tailoringConfig := options.TailoringConfig
	if tailoringConfig == nil {
		return nil
	}

	// TODO: don't panic! unfortunately this would involve quite
	// a big refactor and we still need to be a bit defensive here
	if tailoringConfig.TailoringPath == "" {
		panic(fmt.Errorf("The tailoring path for the OpenSCAP remediation config cannot be empty, this is a programming error"))
	}

	if tailoringConfig.JSONFilepath != "" {
		return &OscapAutotailorStageOptions{
			Filepath: tailoringConfig.TailoringPath,
			Config: AutotailorJSONConfig{
				Datastream:        options.Datastream,
				TailoredProfileID: tailoringConfig.TailoredProfileID,
				TailoringFile:     tailoringConfig.JSONFilepath,
			},
		}
	}

	return &OscapAutotailorStageOptions{
		Filepath: tailoringConfig.TailoringPath,
		Config: AutotailorKeyValueConfig{
			Datastream: options.Datastream,
			ProfileID:  options.ProfileID,
			NewProfile: tailoringConfig.TailoredProfileID,
			Selected:   tailoringConfig.Selected,
			Unselected: tailoringConfig.Unselected,
		},
	}
}
