package osbuild

type FirewallStageOptions struct {
	Ports            []string `json:"ports,omitempty"`
	EnabledServices  []string `json:"enabled_services,omitempty"`
	DisabledServices []string `json:"disabled_services,omitempty"`
	DefaultZone      string   `json:"default_zone,omitempty"`
}

func (FirewallStageOptions) isStageOptions() {}

func NewFirewallStage(options *FirewallStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.firewall",
		Options: options,
	}
}
