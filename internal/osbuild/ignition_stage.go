package osbuild

type IgnitionStageOptions struct {
}

func (IgnitionStageOptions) isStageOptions() {}

func NewIgnitionStage(options *IgnitionStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ignition",
		Options: options,
	}
}
