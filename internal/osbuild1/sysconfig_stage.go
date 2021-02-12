package osbuild1

type SysconfigStageOptions struct {
	Kernel  SysconfigKernelOptions  `json:"kernel,omitempty"`
	Network SysconfigNetworkOptions `json:"network,omitempty"`
}

type SysconfigNetworkOptions struct {
	Networking bool `json:"networking,omitempty"`
	NoZeroConf bool `json:"no_zero_conf,omitempty"`
}

type SysconfigKernelOptions struct {
	UpdateDefault bool   `json:"update_default,omitempty"`
	DefaultKernel string `json:"default_kernel,omitempty"`
}

func (SysconfigStageOptions) isStageOptions() {}

func NewSysconfigStage(options *SysconfigStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.sysconfig",
		Options: options,
	}
}
