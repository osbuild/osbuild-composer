package osbuild1

type FirstBootStageOptions struct {
	Commands       []string `json:"commands"`
	WaitForNetwork bool     `json:"wait_for_network,omitempty"`
}

func (FirstBootStageOptions) isStageOptions() {}

func NewFirstBootStage(options *FirstBootStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.first-boot",
		Options: options,
	}
}
