package osbuild1

type ChronyStageOptions struct {
	Timeservers []string `json:"timeservers"`
}

func (ChronyStageOptions) isStageOptions() {}

func NewChronyStage(options *ChronyStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.chrony",
		Options: options,
	}
}
