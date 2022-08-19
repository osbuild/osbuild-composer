package osbuild

type GreenbootOptions struct {
	Config *GreenbootConfig `json:"config,omitempty"`
}

type GreenbootConfig struct {
	MonitorServices []string `json:"monitor_services,omitempty"`
}

func (GreenbootOptions) isStageOptions() {}

func NewGreenbootConfig(options *GreenbootOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.greenboot",
		Options: options,
	}
}
