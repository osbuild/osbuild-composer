package osbuild

type XorrisofsStageOptions struct {
	// Filename of the ISO to create
	Filename string `json:"filename"`

	// Volume ID to set
	VolID string `json:"volid"`

	// System ID to set
	SysID string `json:"sysid,omitempty"`

	// Publisher to set
	Pub string `json:"pub,omitempty"`

	// Application ID to set
	AppID string `json:"appid,omitempty"`

	// Preparer to set
	Prep string `json:"prep,omitempty"`

	Boot *XorrisofsBoot `json:"boot,omitempty"`

	EFI string `json:"efi,omitempty"`

	// Install the argument (buildroot) as ISOLINUX isohybrid MBR
	IsohybridMBR string `json:"isohybridmbr,omitempty"`

	// The ISO 9660 version (limits data size and filenames; min: 1, max: 4)
	ISOLevel int `json:"isolevel,omitempty"`

	// Path to grub2 hybrid mbr boot image
	// This will cause the created iso to use grub2 instead of syslinux/isolinux
	// when booting on BIOS systems.
	Grub2MBR string `json:"grub2mbr,omitempty"`
}

type XorrisofsBoot struct {
	// Path to the boot image (on the ISO)
	Image string `json:"image"`
	// Path to the boot catalog file (on the ISO)
	Catalog string `json:"catalog"`
}

func (XorrisofsStageOptions) isStageOptions() {}

// Assembles a Rock Ridge enhanced ISO 9660 filesystem (iso)
func NewXorrisofsStage(options *XorrisofsStageOptions, inputPipeline string) *Stage {
	return &Stage{
		Type:    "org.osbuild.xorrisofs",
		Options: options,
		Inputs:  NewPipelineTreeInputs("tree", inputPipeline),
	}
}
