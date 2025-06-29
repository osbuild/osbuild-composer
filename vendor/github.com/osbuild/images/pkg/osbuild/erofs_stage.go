package osbuild

type ErofsCompression struct {
	Method string `json:"method"`
	Level  *int   `json:"level,omitempty"`
}

type ErofsStageOptions struct {
	Filename     string   `json:"filename"`
	ExcludePaths []string `json:"exclude_paths,omitempty"`

	Compression     *ErofsCompression `json:"compression,omitempty"`
	ExtendedOptions []string          `json:"options,omitempty"`
	ClusterSize     *int              `json:"cluster-size,omitempty"`
}

func (ErofsStageOptions) isStageOptions() {}

func NewErofsStage(options *ErofsStageOptions, inputPipeline string) *Stage {
	return &Stage{
		Type:    "org.osbuild.erofs",
		Options: options,
		Inputs:  NewPipelineTreeInputs("tree", inputPipeline),
	}
}
