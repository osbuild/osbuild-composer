package target

type LocalTargetOptions struct {
	Location string `json:"location"`
}

func (LocalTargetOptions) isTargetOptions() {}

func NewLocalTarget(options *LocalTargetOptions) *Target {
	return &Target{
		Name:    "org.osbuild.local",
		Options: options,
	}
}
