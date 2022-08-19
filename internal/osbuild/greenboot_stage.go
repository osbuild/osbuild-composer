package osbuild

type GreenbootConfig struct {
	MonitorService []string `json:"monitor_services,omitempty"`
}

func (GreenbootConfig) isStageOptions() {}

func NewGreenbootConfig(option *GreenbootConfig) *Stage {
	return &Stage{
		Type:    "org.osbuild.greenboot",
		Options: option,
	}
}
