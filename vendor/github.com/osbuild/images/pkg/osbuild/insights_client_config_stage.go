package osbuild

type InsightsClientConfigStageOptions struct {
	Proxy string `json:"proxy,omitempty"`
	Path  string `json:"path,omitempty"`
}

func (InsightsClientConfigStageOptions) isStageOptions() {}

func NewInsightsClientConfigStage(options *InsightsClientConfigStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.insights-client.config",
		Options: options,
	}
}
