package osbuild

type BootISOMonoStageOptions struct {
	Product Product `json:"product"`

	Kernel string `json:"kernel"`

	ISOLabel string `json:"isolabel"`

	EFI EFI `json:"efi,omitempty"`

	ISOLinux ISOLinux `json:"isolinux,omitempty"`

	// Additional kernel boot options
	KernelOpts string `json:"kernel_opts,omitempty"`

	Templates string `json:"templates,omitempty"`

	RootFS RootFS `json:"rootfs,omitempty"`
}

type EFI struct {
	Architectures []string `json:"architectures"`
	Vendor        string   `json:"vendor"`
}

type ISOLinux struct {
	Enabled bool `json:"enabled"`
	Debug   bool `json:"debug,omitempty"`
}

type RootFS struct {
	Compression FSCompression `json:"compression"`

	// Size in MiB
	Size int `json:"size"`
}

type FSCompression struct {
	Method  string                `json:"method"`
	Options *FSCompressionOptions `json:"options,omitempty"`
}

type FSCompressionOptions struct {
	BCJ string `json:"bcj"`
}

// BCJOption returns the appropriate xz branch/call/jump (BCJ) filter for the
// given architecture
func BCJOption(arch string) string {
	switch arch {
	case "x86_64":
		return "x86"
	case "aarch64":
		return "arm"
	case "ppc64le":
		return "powerpc"
	}
	return ""
}

func (BootISOMonoStageOptions) isStageOptions() {}

type BootISOMonoStageInputs struct {
	RootFS *BootISOMonoStageInput `json:"rootfs"`
	Kernel *BootISOMonoStageInput `json:"kernel,omitempty"`
}

func (BootISOMonoStageInputs) isStageInputs() {}

type BootISOMonoStageInput struct {
	inputCommon
	References BootISOMonoStageReferences `json:"references"`
}

func (BootISOMonoStageInput) isStageInput() {}

type BootISOMonoStageReferences []string

func (BootISOMonoStageReferences) isReferences() {}

// Assemble a file system tree for a bootable ISO
func NewBootISOMonoStage(options *BootISOMonoStageOptions, inputs *BootISOMonoStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.bootiso.mono",
		Options: options,
		Inputs:  inputs,
	}
}

func NewBootISOMonoStagePipelineTreeInputs(pipeline string) *BootISOMonoStageInputs {
	rootfsInput := new(BootISOMonoStageInput)
	rootfsInput.Type = "org.osbuild.tree"
	rootfsInput.Origin = "org.osbuild.pipeline"
	rootfsInput.References = BootISOMonoStageReferences{"name:" + pipeline}
	return &BootISOMonoStageInputs{
		RootFS: rootfsInput,
	}
}
