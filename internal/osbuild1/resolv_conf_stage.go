package osbuild1

type ResolvConfStageOptions struct {
	Nameserver []string `json:"nameserver,omitempty"`
	Search     []string `json:"search,omitempty"`
}

func (ResolvConfStageOptions) isStageOptions() {}

func NewResolvConfStage(options *ResolvConfStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.resolv-conf",
		Options: options,
	}
}
