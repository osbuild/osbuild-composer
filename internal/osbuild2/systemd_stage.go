package osbuild2

type SystemdStageOptions struct {
	EnabledServices  []string `json:"enabled_services,omitempty"`
	DisabledServices []string `json:"disabled_services,omitempty"`
	DefaultTarget    string   `json:"default_target,omitempty"`

	// For now we support only .service drop-ins, but this may change in the future
	UnitDropins map[string]SystemdServiceUnitDropins `json:"unit_dropins,omitempty"`
}

func (SystemdStageOptions) isStageOptions() {}

func NewSystemdStage(options *SystemdStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.systemd",
		Options: options,
	}
}

// Drop-in configurations for a '.service' unit
type SystemdServiceUnitDropins map[string]SystemdServiceUnitDropin

// Drop-in configuration for a '.service' unit
type SystemdServiceUnitDropin struct {
	Service *SystemdUnitServiceSection `json:"Service,omitempty"`
}

// 'Service' configuration section of a unit file
type SystemdUnitServiceSection struct {
	// Sets environment variables for executed process
	Environment string `json:"Environment,omitempty"`
}
