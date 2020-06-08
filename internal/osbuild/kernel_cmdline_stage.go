package osbuild

// KernelCmdlineStageOptions describe how to create kernel-cmdline stage
//
// Configures the kernel boot parameters, also known as the kernel command line.
type KernelCmdlineStageOptions struct {
	RootFsUUID int `json:"root_fs_uuid,omitempty"`
	KernelOpts int `json:"kernel_opts,omitempty"`
}

func (KernelCmdlineStageOptions) isStageOptions() {}

// NewKernelCmdlineStage creates a new kernel-cmdline Stage object.
func NewKernelCmdlineStage(options *KernelCmdlineStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.kernel-cmdline",
		Options: options,
	}
}
