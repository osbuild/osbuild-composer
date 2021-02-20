package osbuild2

// Inputs for ostree commits
type OSTreeInput struct {
	inputCommon
}

func (OSTreeInput) isInput() {}

func NewOSTreeInput() *OSTreeInput {
	input := new(OSTreeInput)
	input.Type = "org.osbuild.ostree"
	input.Origin = "org.osbuild.source"
	return input
}
