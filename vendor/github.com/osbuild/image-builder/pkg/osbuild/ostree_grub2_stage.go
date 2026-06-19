package osbuild

type OSTreeGrub2StageOptions struct {
	Filename string `json:"filename"`
	Source   string `json:"source,omitempty"`
}

func (OSTreeGrub2StageOptions) isStageOptions() {}

func NewOSTreeGrub2Stage(options *OSTreeGrub2StageOptions, inputPipeline string) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.grub2",
		Options: options,
		Inputs:  NewPipelineTreeInputs("tree", inputPipeline),
	}
}

func NewOSTreeGrub2MountsStage(options *OSTreeGrub2StageOptions, inputs Inputs, devices map[string]Device, mounts []Mount) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.grub2",
		Options: options,
		Inputs:  inputs,
		Devices: devices,
		Mounts:  mounts,
	}
}
