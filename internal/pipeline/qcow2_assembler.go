package pipeline

import "github.com/google/uuid"

type QCOW2AssemblerOptions struct {
	Filename           string    `json:"filename"`
	RootFilesystemUUDI uuid.UUID `json:"root_fs_uuid"`
	Size               uint64    `json:"size"`
}

func (QCOW2AssemblerOptions) isAssemblerOptions() {}

func NewQCOW2AssemblerOptions(filename string, rootFilesystemUUID uuid.UUID, size uint64) *QCOW2AssemblerOptions {
	return &QCOW2AssemblerOptions{
		Filename:           filename,
		RootFilesystemUUDI: rootFilesystemUUID,
		Size:               size,
	}
}

func NewQCOW2Assembler(options *QCOW2AssemblerOptions) *Assembler {
	return &Assembler{
		Name:    "org.osbuild.qcow2",
		Options: options,
	}
}
