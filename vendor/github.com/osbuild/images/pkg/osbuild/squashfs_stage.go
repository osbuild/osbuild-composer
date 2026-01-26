package osbuild

type SquashfsStageOptions struct {
	Filename     string   `json:"filename"`
	Source       string   `json:"source,omitempty"`
	ExcludePaths []string `json:"exclude_paths,omitempty"`

	Compression FSCompression `json:"compression"`
}

func (SquashfsStageOptions) isStageOptions() {}

func NewSquashfsStage(options *SquashfsStageOptions, inputPipeline string) *Stage {
	return &Stage{
		Type:    "org.osbuild.squashfs",
		Options: options,
		Inputs:  NewPipelineTreeInputs("tree", inputPipeline),
	}
}

func NewSquashfsWithMountsStage(options *SquashfsStageOptions, inputs Inputs, devices map[string]Device, mounts []Mount) *Stage {
	return &Stage{
		Type:    "org.osbuild.squashfs",
		Options: options,
		Inputs:  inputs,
		Devices: devices,
		Mounts:  mounts,
	}
}
