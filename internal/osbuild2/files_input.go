package osbuild2

// Inputs for individual files

// Provides all the files, named via their content hash, specified
// via `references` in a new directory.
type FilesInput struct {
	inputCommon
}

func (FilesInput) isInput() {}

func NewFilesInput() *FilesInput {
	input := new(FilesInput)
	input.Type = "org.osbuild.files"
	input.Origin = "org.osbuild.source"
	return input
}
