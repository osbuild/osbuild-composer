package osbuild

// Options for the org.osbuild.mks390image stage.
type MkS390ImageStageOptions struct {
	Kernel string `json:"kernel"` // Path to linux kernel
	Initrd string `json:"initrd"` // Path to inittramfs file
	Config string `json:"config"` // Path to prm config file
	Image  string `json:"image"`  // Path of bootable image file to write
}

func (MkS390ImageStageOptions) isStageOptions() {}

// NewMkS390ImageStage creates a new org.osbuild.mks390image stage
func NewMkS390ImageStage(options *MkS390ImageStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.mks390image",
		Options: options,
	}
}
