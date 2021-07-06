package osbuild2

type MkfsFATStageOptions struct {
	VolID   string `json:"volid"`
	Label   string `json:"label,omitempty"`
	FATSize *int   `json:"fat-size,omitempty"`
}

func (MkfsFATStageOptions) isStageOptions() {}

type MkfsFATStageDevices struct {
	Device Device `json:"device"`
}

func (MkfsFATStageDevices) isStageDevices() {}

func NewMkfsFATStage(options *MkfsFATStageOptions, devices *MkfsFATStageDevices) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkfs.fat",
		Options: options,
		Devices: devices,
	}
}
