package osbuild2

type GrubISOStageOptions struct {
	Product Product `json:"product"`

	Kernel string `json:"kernel"`

	ISOLabel string `json:"isolabel"`

	// taken from bootiso.mono, when that goes away we'll move it here
	EFI EFI `json:"efi,omitempty"`

	// Additional kernel boot options
	KernelOpts string `json:"kernel_opts,omitempty"`
}

func (GrubISOStageOptions) isStageOptions() {}

// Assemble a file system tree for a bootable ISO
func NewGrubISOStage(options *GrubISOStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.grub.iso",
		Options: options,
	}
}
