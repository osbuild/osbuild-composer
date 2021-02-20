package osbuild2

type ChronyStageOptions struct {
	Timeservers []string `json:"timeservers"`
}

func (ChronyStageOptions) isStageOptions() {}

func NewChronyStage(options *ChronyStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.chrony",
		Options: options,
	}
}
