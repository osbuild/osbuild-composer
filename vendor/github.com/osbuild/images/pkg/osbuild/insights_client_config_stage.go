package osbuild

type InsightsClientConfig struct {
	Proxy string `json:"proxy,omitempty"`
	Path  string `json:"path,omitempty"`
}

type InsightsClientConfigStageOptions struct {
	Config InsightsClientConfig `json:"config"`
}

func (InsightsClientConfigStageOptions) isStageOptions() {}

func NewInsightsClientConfigStage(options *InsightsClientConfigStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.insights-client.config",
		Options: options,
	}
}
