// Disk package contains abstract data-types to define disk-related entities.
//
// PartitionTable, Partition and Filesystem types are currently defined.
// All of them can be 1:1 converted to osbuild.QEMUAssemblerOptions.
package disk

import (
	"encoding/hex"
	"io"

	"github.com/google/uuid"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild1"
)

const (
	// Default sector size in bytes
	DefaultSectorSize = 512

	DefaultGrainBytes = uint64(1024 * 1024) // 1 MiB
)

// FSSpec for a filesystem (UUID and Label); the first field of fstab(5)
type FSSpec struct {
	UUID  string
	Label string
}

type FSTabOptions struct {
	// The fourth field of fstab(5); fs_mntops
	MntOps string
	// The fifth field of fstab(5); fs_freq
	Freq uint64
	// The sixth field of fstab(5); fs_passno
	PassNo uint64
}

type Partition struct {
	Start    uint64 // Start of the partition in bytes
	Size     uint64 // Size of the partition in bytes
	Type     string // Partition type, e.g. 0x83 for MBR or a UUID for gpt
	Bootable bool   // `Legacy BIOS bootable` (GPT) or `active` (DOS) flag
	// ID of the partition, dos doesn't use traditional UUIDs, therefore this
	// is just a string.
	UUID string
	// If nil, the partition is raw; It doesn't contain a filesystem.
	Filesystem *Filesystem
}

type Filesystem struct {
	Type string
	// ID of the filesystem, vfat doesn't use traditional UUIDs, therefore this
	// is just a string.
	UUID       string
	Label      string
	Mountpoint string
	// The fourth field of fstab(5); fs_mntops
	FSTabOptions string
	// The fifth field of fstab(5); fs_freq
	FSTabFreq uint64
	// The sixth field of fstab(5); fs_passno
	FSTabPassNo uint64
}

// Ensure the partition has at least the given size. Will do nothing
// if the partition is already larger. Returns if the size changed.
func (p *Partition) EnsureSize(s uint64) bool {
	if s > p.Size {
		p.Size = s
		return true
	}
	return false
}

// Converts Partition to osbuild.QEMUPartition that encodes the same partition.
func (p *Partition) QEMUPartition() osbuild.QEMUPartition {
	var fs *osbuild.QEMUFilesystem
	if p.Filesystem != nil {
		f := p.Filesystem.QEMUFilesystem()
		fs = &f
	}
	return osbuild.QEMUPartition{
		Start:      p.Start,
		Size:       p.Size,
		Type:       p.Type,
		Bootable:   p.Bootable,
		UUID:       p.UUID,
		Filesystem: fs,
	}
}

// Filesystem related functions

// Clone the filesystem structure
func (fs *Filesystem) Clone() *Filesystem {
	if fs == nil {
		return nil
	}

	return &Filesystem{
		Type:         fs.Type,
		UUID:         fs.UUID,
		Label:        fs.Label,
		Mountpoint:   fs.Mountpoint,
		FSTabOptions: fs.FSTabOptions,
		FSTabFreq:    fs.FSTabFreq,
		FSTabPassNo:  fs.FSTabPassNo,
	}
}

// Converts Filesystem to osbuild.QEMUFilesystem that encodes the same fs.
func (fs *Filesystem) QEMUFilesystem() osbuild.QEMUFilesystem {
	return osbuild.QEMUFilesystem{
		Type:       fs.Type,
		UUID:       fs.UUID,
		Label:      fs.Label,
		Mountpoint: fs.Mountpoint,
	}
}

// uuid generator helpers

// GeneratesnewRandomUUIDFromReader generates a new random UUID (version
// 4 using) via the given random number generator.
func newRandomUUIDFromReader(r io.Reader) (uuid.UUID, error) {
	var id uuid.UUID
	_, err := io.ReadFull(r, id[:])
	if err != nil {
		return uuid.Nil, err
	}
	id[6] = (id[6] & 0x0f) | 0x40 // Version 4
	id[8] = (id[8] & 0x3f) | 0x80 // Variant is 10
	return id, nil
}

// NewRandomVolIDFromReader creates a random 32 bit hex string to use as a
// volume ID for FAT filesystems
func NewRandomVolIDFromReader(r io.Reader) (string, error) {
	volid := make([]byte, 4)
	_, err := r.Read(volid)
	return hex.EncodeToString(volid), err
}
