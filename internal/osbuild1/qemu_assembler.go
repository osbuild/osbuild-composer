package osbuild1

// QEMUAssemblerOptions desrcibe how to assemble a tree into an image using qemu.
//
// The assembler creates an image of the given size, adds a GRUB2 bootloader
// and if necessary and a partition table to it with the given PTUUID
// containing the indicated partitions. Finally, the image is converted into
// the target format and stored with the given filename.
type QEMUAssemblerOptions struct {
	Bootloader *QEMUBootloader `json:"bootloader,omitempty"`
	Format     string          `json:"format"`
	Filename   string          `json:"filename"`
	Size       uint64          `json:"size"`
	PTUUID     string          `json:"ptuuid"`
	PTType     string          `json:"pttype"`
	Partitions []QEMUPartition `json:"partitions"`
}

type QEMUPartition struct {
	Start      uint64          `json:"start"`
	Size       uint64          `json:"size,omitempty"`
	Type       string          `json:"type,omitempty"`
	Bootable   bool            `json:"bootable,omitempty"`
	UUID       string          `json:"uuid,omitempty"`
	Filesystem *QEMUFilesystem `json:"filesystem,omitempty"`
}

type QEMUFilesystem struct {
	Type       string `json:"type"`
	UUID       string `json:"uuid"`
	Label      string `json:"label,omitempty"`
	Mountpoint string `json:"mountpoint"`
}

type QEMUBootloader struct {
	Type     string `json:"type,omitempty"`
	Platform string `json:"platform,omitempty"`
}

func (QEMUAssemblerOptions) isAssemblerOptions() {}

// NewQEMUAssembler creates a new QEMU Assembler object.
func NewQEMUAssembler(options *QEMUAssemblerOptions) *Assembler {
	return &Assembler{
		Name:    "org.osbuild.qemu",
		Options: options,
	}
}
