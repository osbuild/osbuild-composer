package osbuild

type GreenbootConfig struct {
	MonitorServices []string `json:"monitor_services,omitempty"`
}

func (GreenbootConfig) isStageOptions() {}

func NewGreenbootConfig(options *GreenbootConfig) *Stage {
	return &Stage{
		Type:    "org.osbuild.greenboot",
		Options: options,
	}
}
