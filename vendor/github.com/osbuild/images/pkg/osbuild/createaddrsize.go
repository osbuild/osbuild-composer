package osbuild

// Options for the org.osbuild.createaddrsize stage.
type CreateaddrsizeStageOptions struct {
	Initrd   string `json:"initrd"`   // Path to inittramfs file
	Addrsize string `json:"addrsize"` // Path of addrsize file to write
}

func (CreateaddrsizeStageOptions) isStageOptions() {}

// NewCreateaddrsizeStage creates a new org.osbuild.createaddrsize stage
func NewCreateaddrsizeStage(options *CreateaddrsizeStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.createaddrsize",
		Options: options,
	}
}
