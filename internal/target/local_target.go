package target

type LocalTargetOptions struct {
	Location string `json:"location"`
}

func (LocalTargetOptions) isTargetOptions() {}

func NewLocalTargetOptions(location string) *LocalTargetOptions {
	return &LocalTargetOptions{
		Location: location,
	}
}

func NewLocalTarget(options *LocalTargetOptions) *Target {
	return &Target{
		Name:    "org.osbuild.local",
		Options: options,
	}
}
