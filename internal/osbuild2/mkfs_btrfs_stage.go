package osbuild2

type MkfsBtrfsStageOptions struct {
	UUID  string `json:"uuid"`
	Label string `json:"label,omitempty"`
}

func (MkfsBtrfsStageOptions) isStageOptions() {}

func NewMkfsBtrfsStage(options *MkfsBtrfsStageOptions, device *Device) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkfs.btrfs",
		Options: options,
		Devices: Devices{"device": *device},
	}
}
