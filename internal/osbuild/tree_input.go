package osbuild

// Tree inputs
type TreeInput struct {
	inputCommon
}

func (TreeInput) isInput() {}

func NewTreeInput() *TreeInput {
	input := new(TreeInput)
	input.Type = "org.osbuild.tree"
	input.Origin = "org.osbuild.pipeline"
	return input
}
