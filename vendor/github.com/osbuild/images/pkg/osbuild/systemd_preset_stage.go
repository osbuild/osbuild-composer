package osbuild

import "fmt"

type SystemdPresetStageOptions struct {
	Presets []Preset `json:"presets,omitempty"`
}

type PresetState string

const (
	StateEnable  PresetState = "enable"
	StateDisable PresetState = "disable"
)

type Preset struct {
	Name  string      `json:"name,omitempty"`
	State PresetState `json:"state,omitempty"`
}

func (SystemdPresetStageOptions) isStageOptions() {}

func NewSystemdPresetStage(options *SystemdPresetStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}
	return &Stage{
		Type:    "org.osbuild.systemd.preset",
		Options: options,
	}
}

func (o SystemdPresetStageOptions) validate() error {
	if len(o.Presets) == 0 {
		return fmt.Errorf("at least one preset is required")
	}
	return nil
}
