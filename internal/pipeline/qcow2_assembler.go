package pipeline

import "github.com/google/uuid"

// QEMUAssemblerOptions desrcibe how to assemble a tree into an image using qemu.
//
// The assembler creates an image of a the given size, adds a GRUB2 bootloader
// and a DOS partition table to it with the given PTUUID containing one ext4
// root partition with the given filesystem UUID and installs the filesystem
// tree into it. Finally, the image is converted into the target format and
// stored with the given filename.
type QEMUAssemblerOptions struct {
	Format             string    `json:"format"`
	Filename           string    `json:"filename"`
	PTUUID             string    `json:"ptuuid"`
	RootFilesystemUUDI uuid.UUID `json:"root_fs_uuid"`
	Size               uint64    `json:"size"`
}

func (QEMUAssemblerOptions) isAssemblerOptions() {}

// NewQEMUAssemblerOptions creates a now QEMUAssemblerOptions object, with all the mandatory
// fields set.
func NewQEMUAssemblerOptions(format string, ptUUID string, filename string, rootFilesystemUUID uuid.UUID, size uint64) *QEMUAssemblerOptions {
	return &QEMUAssemblerOptions{
		Format:             format,
		PTUUID:             ptUUID,
		Filename:           filename,
		RootFilesystemUUDI: rootFilesystemUUID,
		Size:               size,
	}
}

// NewQEMUAssembler creates a new QEMU Assembler object.
func NewQEMUAssembler(options *QEMUAssemblerOptions) *Assembler {
	return &Assembler{
		Name:    "org.osbuild.qcow2",
		Options: options,
	}
}
