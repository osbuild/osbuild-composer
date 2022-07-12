package osbuild

// Single stage of a pipeline executing one step
type Stage struct {
	// Well-known name in reverse domain-name notation, uniquely identifying
	// the stage type.
	Type string `json:"type"`
	// Stage-type specific options fully determining the operations of the

	Inputs  Inputs       `json:"inputs,omitempty"`
	Options StageOptions `json:"options,omitempty"`
	Devices Devices      `json:"devices,omitempty"`
	Mounts  Mounts       `json:"mounts,omitempty"`
}

// StageOptions specify the operations of a given stage-type.
type StageOptions interface {
	isStageOptions()
}
