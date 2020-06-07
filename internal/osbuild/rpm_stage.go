package osbuild

// The RPMStageOptions describe the operations of the RPM stage.
//
// The RPM stage installs a given set of packages, identified by their
// content hash. This ensures that given a set of RPM stage options,
// the output is be reproducible, if the underlying tools are.
type RPMStageOptions struct {
	GPGKeys  []string     `json:"gpgkeys,omitempty"`
	Packages []RPMPackage `json:"packages"`
}

// RPMPackage represents one RPM, as referenced by its content hash
// (checksum). The files source must indicate where to fetch the given
// RPM. If CheckGPG is `true` the RPM must be signed with one of the
// GPGKeys given in the RPMStageOptions.
type RPMPackage struct {
	Checksum string `json:"checksum"`
	CheckGPG bool   `json:"check_gpg,omitempty"`
}

func (RPMStageOptions) isStageOptions() {}

// NewRPMStage creates a new RPM stage.
func NewRPMStage(options *RPMStageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.rpm",
		Options: options,
	}
}
