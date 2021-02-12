package osbuild1

type FirewallStageOptions struct {
	Ports            []string `json:"ports,omitempty"`
	EnabledServices  []string `json:"enabled_services,omitempty"`
	DisabledServices []string `json:"disabled_services,omitempty"`
}

func (FirewallStageOptions) isStageOptions() {}

func NewFirewallStage(options *FirewallStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.firewall",
		Options: options,
	}
}
