package osbuild

type SystemdUnitStageOptions struct {
	Unit   string                   `json:"unit"`
	Dropin string                   `json:"dropin"`
	Config SystemdServiceUnitDropin `json:"config"`
}

func (SystemdUnitStageOptions) isStageOptions() {}

func NewSystemdUnitStage(options *SystemdUnitStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.systemd.unit",
		Options: options,
	}
}

// Drop-in configuration for a '.service' unit
type SystemdServiceUnitDropin struct {
	Service *SystemdUnitServiceSection `json:"Service,omitempty"`
}

// 'Service' configuration section of a unit file
type SystemdUnitServiceSection struct {
	// Sets environment variables for executed process
	Environment string `json:"Environment,omitempty"`
}
