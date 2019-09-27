package pipeline

type FixBLSStageOptions struct {
}

func (FixBLSStageOptions) isStageOptions() {}

func NewFIXBLSStage() *Stage {
	return &Stage{
		Name: "org.osbuild.fix-bls",
	}
}
