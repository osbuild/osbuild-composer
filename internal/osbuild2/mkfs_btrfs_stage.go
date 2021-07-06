package osbuild2

type MkfsBtrfsStageOptions struct {
	UUID  string `json:"uuid"`
	Label string `json:"label,omitempty"`
}

func (MkfsBtrfsStageOptions) isStageOptions() {}

type MkfsBtrfsStageDevices struct {
	Device Device `json:"device"`
}

func (MkfsBtrfsStageDevices) isStageDevices() {}

func NewMkfsBtrfsStage(options *MkfsBtrfsStageOptions, devices *MkfsBtrfsStageDevices) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkfs.btrfs",
		Options: options,
		Devices: devices,
	}
}
