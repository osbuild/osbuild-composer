package osbuild1

import "github.com/osbuild/osbuild-composer/internal/disk"

// QEMUAssemblerOptions desrcibe how to assemble a tree into an image using qemu.
//
// The assembler creates an image of the given size, adds a GRUB2 bootloader
// and if necessary and a partition table to it with the given PTUUID
// containing the indicated partitions. Finally, the image is converted into
// the target format and stored with the given filename.
type QEMUAssemblerOptions struct {
	Bootloader  *QEMUBootloader `json:"bootloader,omitempty"`
	Format      string          `json:"format"`
	Qcow2Compat string          `json:"qcow2_compat,omitempty"`
	Filename    string          `json:"filename"`
	Size        uint64          `json:"size"`
	PTUUID      string          `json:"ptuuid"`
	PTType      string          `json:"pttype"`
	Partitions  []QEMUPartition `json:"partitions"`
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

// NewQEMUAssemblerOptions creates and returns QEMUAssemblerOptions based on
// the given PartitionTable.
func NewQEMUAssemblerOptions(pt *disk.PartitionTable) QEMUAssemblerOptions {
	var partitions []QEMUPartition
	for idx := range pt.Partitions {
		partitions = append(partitions, NewQEMUPartition(&pt.Partitions[idx]))
	}

	return QEMUAssemblerOptions{
		Size:       pt.Size,
		PTUUID:     pt.UUID,
		PTType:     pt.Type,
		Partitions: partitions,
	}
}

// NewQEMUPartition creates and returns a QEMUPartition based on the given
// Partition.
func NewQEMUPartition(p *disk.Partition) QEMUPartition {
	var fs *QEMUFilesystem
	if p.Payload != nil {
		f := NewQEMUFilesystem(p.Payload)
		fs = &f
	}
	return QEMUPartition{
		Start:      p.Start,
		Size:       p.Size,
		Type:       p.Type,
		Bootable:   p.Bootable,
		UUID:       p.UUID,
		Filesystem: fs,
	}
}

// NewQEMUFilesystem creates and returns a QEMUFilesystem based on the given
// Filesystem.
func NewQEMUFilesystem(fs *disk.Filesystem) QEMUFilesystem {
	return QEMUFilesystem{
		Type:       fs.Type,
		UUID:       fs.UUID,
		Label:      fs.Label,
		Mountpoint: fs.Mountpoint,
	}
}
