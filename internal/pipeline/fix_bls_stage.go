package pipeline

type FixBLSStageOptions struct {
}

func (FixBLSStageOptions) isStageOptions() {}

func NewFixBLSStage() *Stage {
	return &Stage{
		Name: "org.osbuild.fix-bls",
	}
}
