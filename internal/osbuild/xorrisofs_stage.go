package osbuild

type XorrisofsStageOptions struct {
	// Filename of the ISO to create
	Filename string `json:"filename"`

	// Volume ID to set
	VolID string `json:"volid"`

	// System ID to set
	SysID string `json:"sysid,omitempty"`

	Boot *XorrisofsBoot `json:"boot,omitempty"`

	EFI string `json:"efi,omitempty"`

	// Install the argument (buildroot) as ISOLINUX isohybrid MBR
	IsohybridMBR string `json:"isohybridmbr,omitempty"`

	// The ISO 9660 version (limits data size and filenames; min: 1, max: 4)
	ISOLevel int `json:"isolevel,omitempty"`
}

type XorrisofsBoot struct {
	// Path to the boot image (on the ISO)
	Image string `json:"image"`
	// Path to the boot catalog file (on the ISO)
	Catalog string `json:"catalog"`
}

func (XorrisofsStageOptions) isStageOptions() {}

type XorrisofsStageInputs struct {
	Tree *XorrisofsStageInput `json:"tree"`
}

func (XorrisofsStageInputs) isStageInputs() {}

type XorrisofsStageInput struct {
	inputCommon
	References XorrisofsStageReferences `json:"references"`
}

func (XorrisofsStageInput) isStageInput() {}

type XorrisofsStageReferences []string

func (XorrisofsStageReferences) isReferences() {}

// Assembles a Rock Ridge enhanced ISO 9660 filesystem (iso)
func NewXorrisofsStage(options *XorrisofsStageOptions, inputs *XorrisofsStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.xorrisofs",
		Options: options,
		Inputs:  inputs,
	}
}

func NewXorrisofsStagePipelineTreeInputs(pipeline string) *XorrisofsStageInputs {
	input := new(XorrisofsStageInput)
	input.Type = "org.osbuild.tree"
	input.Origin = "org.osbuild.pipeline"
	input.References = XorrisofsStageReferences{"name:" + pipeline}
	return &XorrisofsStageInputs{Tree: input}
}
