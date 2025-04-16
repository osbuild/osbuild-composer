package osbuild

type WAAgentConfig struct {
	ProvisioningUseCloudInit *bool `json:"Provisioning.UseCloudInit,omitempty"`
	ProvisioningEnabled      *bool `json:"Provisioning.Enabled,omitempty"`
	RDFormat                 *bool `json:"ResourceDisk.Format,omitempty"`
	RDEnableSwap             *bool `json:"ResourceDisk.EnableSwap,omitempty"`
}

type WAAgentConfStageOptions struct {
	Config WAAgentConfig `json:"config"`
}

func (WAAgentConfStageOptions) isStageOptions() {}

func NewWAAgentConfStage(options *WAAgentConfStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.waagent.conf",
		Options: options,
	}
}
