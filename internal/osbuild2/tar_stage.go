package osbuild2

type TarStageOptions struct {
	// Filename for tar archive
	Filename string `json:"filename"`

	// Enable support for POSIX ACLs
	ACLs bool `json:"acls,omitempty"`

	// Enable support for SELinux contexts
	SELinux bool `json:"selinux,omitempty"`

	// Enable support for extended attributes
	Xattrs bool `json:"xattrs,omitempty"`
}

func (TarStageOptions) isStageOptions() {}

type TarStageInput struct {
	inputCommon
	References TarStageReferences `json:"references"`
}

func (TarStageInput) isStageInput() {}

type TarStageInputs struct {
	Tree *TarStageInput `json:"tree"`
}

func (TarStageInputs) isStageInputs() {}

type TarStageReferences []string

func (TarStageReferences) isReferences() {}

// Assembles a tree into a tar archive. Compression is determined by the suffix
// (i.e., --auto-compress is used).
func NewTarStage(options *TarStageOptions, inputs *TarStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.tar",
		Options: options,
		Inputs:  inputs,
	}
}
