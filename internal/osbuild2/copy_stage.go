package osbuild2

// Stage to copy items from inputs to mount points or the tree. Multiple items
// can be copied. The source and destination is a URL.

type CopyStageOptions struct {
	Paths []CopyStagePath `json:"paths"`
}

type CopyStagePath struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (CopyStageOptions) isStageOptions() {}

type CopyStageInputs map[string]CopyStageInput

type CopyStageInput struct {
	inputCommon
	References CopyStageReferences `json:"references"`
}

func (CopyStageInputs) isStageInputs() {}

type CopyStageReferences []string

func (CopyStageReferences) isReferences() {}

func NewCopyStage(options *CopyStageOptions, inputs *CopyStageInputs, devices *Devices, mounts *Mounts) *Stage {
	return &Stage{
		Type:    "org.osbuild.copy",
		Options: options,
		Inputs:  inputs,
		Devices: *devices,
		Mounts:  *mounts,
	}
}
