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

type CopyStageInputsNew interface {
	isCopyStageInputs()
}

func (CopyStageInputs) isCopyStageInputs() {}

func (CopyStageReferences) isReferences() {}

func NewCopyStage(options *CopyStageOptions, inputs CopyStageInputsNew, devices *Devices, mounts *Mounts) *Stage {
	var stageInputs Inputs
	if inputs != nil {
		stageInputs = inputs.(Inputs)
	}
	return &Stage{
		Type:    "org.osbuild.copy",
		Options: options,
		Inputs:  stageInputs,
		Devices: *devices,
		Mounts:  *mounts,
	}
}
