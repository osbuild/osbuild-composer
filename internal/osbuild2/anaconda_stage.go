package osbuild2

type AnacondaStageOptions struct {
	// Kickstart modules to enable
	KickstartModules []string `json:"kickstart-modules"`
}

func (AnacondaStageOptions) isStageOptions() {}

// Configure basic aspects of the Anaconda installer
func NewAnacondaStage(options *AnacondaStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.anaconda",
		Options: options,
	}
}
