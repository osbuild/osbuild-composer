package osbuild

type FirewallStageOptions struct {
	Ports            []string         `json:"ports,omitempty"`
	EnabledServices  []string         `json:"enabled_services,omitempty"`
	DisabledServices []string         `json:"disabled_services,omitempty"`
	DefaultZone      string           `json:"default_zone,omitempty"`
	Sources          []FirewallSource `json:"sources,omitempty"`
}

type FirewallSource struct {
	Zone    string   `json:"zone,omitempty"`
	Sources []string `json:"sources,omitempty"`
}

func (FirewallStageOptions) isStageOptions() {}

func NewFirewallStage(options *FirewallStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.firewall",
		Options: options,
	}
}
