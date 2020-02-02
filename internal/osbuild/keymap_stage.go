package osbuild

type KeymapStageOptions struct {
	Keymap string `json:"keymap"`
}

func (KeymapStageOptions) isStageOptions() {}

func NewKeymapStage(options *KeymapStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.keymap",
		Options: options,
	}
}
