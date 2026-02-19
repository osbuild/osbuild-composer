package osbuild

type ErofsCompression struct {
	Method string `json:"method" yaml:"method"`
	Level  *int   `json:"level,omitempty" yaml:"level,omitempty"`
}

type ErofsStageOptions struct {
	Filename     string   `json:"filename" yaml:"filename"`
	Source       string   `json:"source,omitempty" yaml:"source,omitempty"`
	ExcludePaths []string `json:"exclude_paths,omitempty" yaml:"exclude_paths,omitempty"`

	Compression     *ErofsCompression `json:"compression,omitempty" yaml:"compression,omitempty"`
	ExtendedOptions []string          `json:"options,omitempty" yaml:"options,omitempty"`
	ClusterSize     *int              `json:"cluster-size,omitempty" yaml:"cluster-size,omitempty"`
}

func (ErofsStageOptions) isStageOptions() {}

func NewErofsStage(options ErofsStageOptions, inputPipeline string) *Stage {
	opts := options
	return &Stage{
		Type:    "org.osbuild.erofs",
		Options: &opts,
		Inputs:  NewPipelineTreeInputs("tree", inputPipeline),
	}
}

func NewErofsWithMountsStage(options *ErofsStageOptions, inputs Inputs, devices map[string]Device, mounts []Mount) *Stage {
	return &Stage{
		Type:    "org.osbuild.erofs",
		Options: options,
		Inputs:  inputs,
		Devices: devices,
		Mounts:  mounts,
	}
}
