package osbuild

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
	return &Stage{
		Type:    "org.osbuild.systemd.preset",
		Options: options,
	}
}
