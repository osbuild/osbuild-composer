package osbuild2

type OCIArchiveStageOptions struct {
	// The CPU architecture of the image
	Architecture string `json:"architecture"`

	// Resulting image filename
	Filename string `json:"filename"`

	// The execution parameters
	Config *OCIArchiveConfig `json:"config,omitempty"`
}

type OCIArchiveConfig struct {
	Cmd          []string          `json:"Cmd,omitempty"`
	Env          []string          `json:"Env,omitempty"`
	ExposedPorts []string          `json:"ExposedPorts,omitempty"`
	User         string            `json:"User,omitempty"`
	Labels       map[string]string `json:"Labels,omitempty"`
	StopSignal   string            `json:"StopSignal,omitempty"`
	Volumes      []string          `json:"Volumes,omitempty"`
	WorkingDir   string            `json:"WorkingDir,omitempty"`
}

func (OCIArchiveStageOptions) isStageOptions() {}

type OCIArchiveStageInputs struct {
	Base *OCIArchiveStageInput `json:"base"`
}

func (OCIArchiveStageInputs) isStageInputs() {}

type OCIArchiveStageInput struct {
	inputCommon
	References OCIArchiveStageReferences `json:"references"`
}

func (OCIArchiveStageInput) isStageInput() {}

type OCIArchiveStageReferences []string

func (OCIArchiveStageReferences) isReferences() {}

// A new OCIArchiveStage to to assemble an OCI image archive
func NewOCIArchiveStage(options *OCIArchiveStageOptions, inputs *OCIArchiveStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.oci-archive",
		Options: options,
		Inputs:  inputs,
	}
}
