package pipeline

type SystemdStageOptions struct {
	EnabledServices  []string `json:"enabled_services,omitempty"`
	DisabledServices []string `json:"disabled_services,omitempty"`
}

func (SystemdStageOptions) isStageOptions() {}

func NewSystemdStage(options *SystemdStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.systemd",
		Options: options,
	}
}
