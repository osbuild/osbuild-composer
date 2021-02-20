package osbuild2

type RPMStageOptions struct {
	// Array of GPG key contents to import
	GPGKeys []string `json:"gpgkeys,omitempty"`

	// Prevent dracut from running
	DisableDracut bool `json:"disable_dracut,omitempty"`

	Exclude *Exclude `json:"exclude,omitempty"`
}

type Exclude struct {
	// Do not install documentation.
	Docs bool `json:"docs,omitempty"`
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

type RPMStageInputs struct {
	Packages *RPMStageInput `json:"packages"`
}

func (RPMStageInputs) isStageInputs() {}

type RPMStageInput struct {
	inputCommon
	References RPMStageReferences `json:"references"`
}

func (RPMStageInput) isStageInput() {}

type RPMStageReferences []string

func (RPMStageReferences) isReferences() {}

// NewRPMStage creates a new RPM stage.
func NewRPMStage(options *RPMStageOptions, inputs *RPMStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.rpm",
		Inputs:  inputs,
		Options: options,
	}
}
