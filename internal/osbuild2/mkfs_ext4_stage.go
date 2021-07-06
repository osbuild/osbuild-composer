package osbuild2

type MkfsExt4StageOptions struct {
	UUID  string `json:"uuid"`
	Label string `json:"label,omitempty"`
}

func (MkfsExt4StageOptions) isStageOptions() {}

type MkfsExt4StageDevices struct {
	Device Device `json:"device"`
}

func (MkfsExt4StageDevices) isStageDevices() {}

func NewMkfsExt4Stage(options *MkfsExt4StageOptions, devices *MkfsExt4StageDevices) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkfs.ext4",
		Options: options,
		Devices: devices,
	}
}
