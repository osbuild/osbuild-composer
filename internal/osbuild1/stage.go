package osbuild1

// A Stage transforms a filesystem tree.
type Stage struct {
	// Well-known name in reverse domain-name notation, uniquely identifying
	// the stage type.
	Name string `json:"name"`
	// Stage-type specific options fully determining the operations of the
	// stage.
	Options StageOptions `json:"options"`
}

// StageOptions specify the operations of a given stage-type.
type StageOptions interface {
	isStageOptions()
}
