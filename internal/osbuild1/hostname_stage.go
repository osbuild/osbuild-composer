package osbuild1

type HostnameStageOptions struct {
	Hostname string `json:"hostname"`
}

func (HostnameStageOptions) isStageOptions() {}

func NewHostnameStage(options *HostnameStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.hostname",
		Options: options,
	}
}
