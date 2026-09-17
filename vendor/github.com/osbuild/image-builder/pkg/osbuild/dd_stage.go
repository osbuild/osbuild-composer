package osbuild

type DDStageOptions struct {
	Src       string `json:"src"`
	Dst       string `json:"dst"`
	SrcOffset *int   `json:"src_offset,omitempty"`
	Count     int    `json:"count"`
}

func (DDStageOptions) isStageOptions() {}

type DDStageInputs map[string]TreeInput

func (DDStageInputs) isStageInputs() {}

func NewDDStage(options *DDStageOptions, inputs Inputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.dd",
		Options: options,
		Inputs:  inputs,
	}
}
