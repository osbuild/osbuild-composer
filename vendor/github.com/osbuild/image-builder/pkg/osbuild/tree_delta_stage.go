package osbuild

type TreeDeltaStageOptions struct {
	Paths        []string `json:"paths,omitempty"`
	ExcludePaths []string `json:"exclude_paths,omitempty"`
}

func (TreeDeltaStageOptions) isStageOptions() {}

type TreeDeltaStageInputs struct {
	Reference *TreeInput `json:"reference"`
	Overlay   *TreeInput `json:"overlay"`
}

func (TreeDeltaStageInputs) isStageInputs() {}

func NewTreeDeltaStage(options *TreeDeltaStageOptions, referencePipeline, overlayPipeline string) *Stage {
	return &Stage{
		Type:    "org.osbuild.tree-delta",
		Options: options,
		Inputs: &TreeDeltaStageInputs{
			Reference: NewTreeInput("name:" + referencePipeline),
			Overlay:   NewTreeInput("name:" + overlayPipeline),
		},
	}
}
