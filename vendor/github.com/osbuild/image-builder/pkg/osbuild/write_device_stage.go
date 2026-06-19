package osbuild

type WriteDeviceStageOptions struct {
	From string `json:"from"`
}

func (WriteDeviceStageOptions) isStageOptions() {}

func NewWriteDeviceStage(options *WriteDeviceStageOptions, inputs Inputs, devices map[string]Device) *Stage {
	return &Stage{
		Type:    "org.osbuild.write-device",
		Options: options,
		Inputs:  inputs,
		Devices: devices,
	}
}
