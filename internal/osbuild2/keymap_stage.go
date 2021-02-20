package osbuild2

type KeymapStageOptions struct {
	Keymap string `json:"keymap"`
}

func (KeymapStageOptions) isStageOptions() {}

func NewKeymapStage(options *KeymapStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.keymap",
		Options: options,
	}
}
