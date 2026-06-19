package osbuild

type MkswapStageOptions struct {
	UUID  string `json:"uuid"`
	Label string `json:"label,omitempty"`
}

func (MkswapStageOptions) isStageOptions() {}

func NewMkswapStage(options *MkswapStageOptions, devices map[string]Device) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkswap",
		Options: options,
		Devices: devices,
	}
}
