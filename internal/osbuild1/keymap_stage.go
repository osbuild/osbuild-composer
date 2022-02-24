package osbuild1

type X11Keymap struct {
	Layouts []string `json:"layouts"`
}

type KeymapStageOptions struct {
	Keymap    string     `json:"keymap"`
	X11Keymap *X11Keymap `json:"x11-keymap,omitempty"`
}

func (KeymapStageOptions) isStageOptions() {}

func NewKeymapStage(options *KeymapStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.keymap",
		Options: options,
	}
}
