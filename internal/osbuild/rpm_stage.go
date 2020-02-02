package osbuild

// The RPMStageOptions describe the operations of the RPM stage.
//
// The RPM stage installs a given set of packages, identified by their
// content hash. This ensures that given a set of RPM stage options,
// the output is be reproducible, if the underlying tools are.
type RPMStageOptions struct {
	GPGKeys  []string `json:"gpgkeys,omitempty"`
	Packages []string `json:"packages"`
}

func (RPMStageOptions) isStageOptions() {}

// NewRPMStage creates a new RPM stage.
func NewRPMStage(options *RPMStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.rpm",
		Options: options,
	}
}
