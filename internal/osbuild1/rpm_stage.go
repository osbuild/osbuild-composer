package osbuild1

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

// RPMStageMetadata gives the set of packages installed by the RPM stage
type RPMStageMetadata struct {
	Packages []RPMPackageMetadata `json:"packages"`
}

// RPMPackageMetadata contains the metadata extracted from one RPM header
type RPMPackageMetadata struct {
	Name    string  `json:"name"`
	Version string  `json:"version"`
	Release string  `json:"release"`
	Epoch   *string `json:"epoch"`
	Arch    string  `json:"arch"`
	SigMD5  string  `json:"sigmd5"`
	SigPGP  string  `json:"sigpgp"`
	SigGPG  string  `json:"siggpg"`
}

func (RPMStageMetadata) isStageMetadata() {}
