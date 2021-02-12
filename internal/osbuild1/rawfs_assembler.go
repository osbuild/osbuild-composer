package osbuild1

import "github.com/google/uuid"

// RawFSAssemblerOptions desrcibe how to assemble a tree into a raw filesystem
// image.
type RawFSAssemblerOptions struct {
	Filename           string    `json:"filename"`
	RootFilesystemUUID uuid.UUID `json:"root_fs_uuid"`
	Size               uint64    `json:"size"`
	FilesystemType     string    `json:"fs_type,omitempty"`
}

func (RawFSAssemblerOptions) isAssemblerOptions() {}

// NewRawFSAssembler creates a new RawFS Assembler object.
func NewRawFSAssembler(options *RawFSAssemblerOptions) *Assembler {
	return &Assembler{
		Name:    "org.osbuild.rawfs",
		Options: options,
	}
}
