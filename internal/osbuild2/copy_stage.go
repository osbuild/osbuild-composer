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

func (CopyStageReferences) isReferences() {}

type CopyStageDevices map[string]Device

func (CopyStageDevices) isStageDevices() {}

type CopyStageMounts []Mount

func (CopyStageMounts) isStageMounts() {}

func NewCopyStage(options *CopyStageOptions, inputs *CopyStageInputs, devices *CopyStageDevices, mounts CopyStageMounts) *Stage {
	return &Stage{
		Type:    "org.osbuild.copy",
		Options: options,
		Inputs:  inputs,
		Devices: devices,
		Mounts:  mounts,
	}
}

func NewCopyStageFiles(options *CopyStageOptions, inputs CopyStageInputsNew) *Stage {
	var stageInputs Inputs
	if inputs != nil {
		stageInputs = inputs.(Inputs)
	}
	return &Stage{
		Type:    "org.osbuild.copy",
		Options: options,
		Inputs:  stageInputs,
	}
}
