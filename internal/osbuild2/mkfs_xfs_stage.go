package osbuild2

type MkfsXfsStageOptions struct {
	UUID  string `json:"uuid"`
	Label string `json:"label,omitempty"`
}

func (MkfsXfsStageOptions) isStageOptions() {}

type MkfsXfsStageDevices struct {
	Device Device `json:"device"`
}

func (MkfsXfsStageDevices) isStageDevices() {}

func NewMkfsXfsStage(options *MkfsXfsStageOptions, devices *MkfsXfsStageDevices) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkfs.xfs",
		Options: options,
		Devices: devices,
	}
}
