// Disk package contains abstract data-types to define disk-related entities.
//
// PartitionTable, Partition and Filesystem types are currently defined.
// All of them can be 1:1 converted to osbuild.QEMUAssemblerOptions.
package disk

import (
	"sort"

	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild1"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type PartitionTable struct {
	// Size of the disk.
	Size uint64
	UUID string
	// Partition table type, e.g. dos, gpt.
	Type       string
	Partitions []Partition
}

type Partition struct {
	Start    uint64
	Size     uint64
	Type     string
	Bootable bool
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

// Converts PartitionTable to osbuild.QEMUAssemblerOptions that encode
// the same partition table.
func (pt *PartitionTable) QEMUAssemblerOptions() osbuild.QEMUAssemblerOptions {
	var partitions []osbuild.QEMUPartition
	for _, p := range pt.Partitions {
		partitions = append(partitions, p.QEMUPartition())
	}

	return osbuild.QEMUAssemblerOptions{
		Size:       pt.Size,
		PTUUID:     pt.UUID,
		PTType:     pt.Type,
		Partitions: partitions,
	}
}

// Generates org.osbuild.fstab stage options from this partition table.
func (pt *PartitionTable) FSTabStageOptions() *osbuild.FSTabStageOptions {
	var options osbuild.FSTabStageOptions
	for _, p := range pt.Partitions {
		fs := p.Filesystem
		if fs == nil {
			continue
		}

		options.AddFilesystem(fs.UUID, fs.Type, fs.Mountpoint, fs.FSTabOptions, fs.FSTabFreq, fs.FSTabPassNo)
	}

	// sort the entries by PassNo to maintain backward compatibility
	sort.Slice(options.FileSystems, func(i, j int) bool {
		return options.FileSystems[i].PassNo < options.FileSystems[j].PassNo
	})

	return &options
}

// Generates org.osbuild.fstab stage options from this partition table.
func (pt *PartitionTable) FSTabStageOptionsV2() *osbuild2.FSTabStageOptions {
	var options osbuild2.FSTabStageOptions
	for _, p := range pt.Partitions {
		fs := p.Filesystem
		if fs == nil {
			continue
		}

		options.AddFilesystem(fs.UUID, fs.Type, fs.Mountpoint, fs.FSTabOptions, fs.FSTabFreq, fs.FSTabPassNo)
	}

	// sort the entries by PassNo to maintain backward compatibility
	sort.Slice(options.FileSystems, func(i, j int) bool {
		return options.FileSystems[i].PassNo < options.FileSystems[j].PassNo
	})

	return &options
}

// Returns the root partition (the partition whose filesystem has / as
// a mountpoint) of the partition table. Nil is returned if there's no such
// partition.
func (pt *PartitionTable) RootPartition() *Partition {
	for idx, p := range pt.Partitions {
		if p.Filesystem == nil {
			continue
		}

		if p.Filesystem.Mountpoint == "/" {
			return &pt.Partitions[idx]
		}
	}

	return nil
}

// Returns the /boot partition (the partition whose filesystem has /boot as
// a mountpoint) of the partition table. Nil is returned if there's no such
// partition.
func (pt *PartitionTable) BootPartition() *Partition {
	for _, p := range pt.Partitions {
		if p.Filesystem == nil {
			continue
		}

		if p.Filesystem.Mountpoint == "/boot" {
			return &p
		}
	}

	return nil
}

// Returns the index of the boot partition: the partition whose filesystem has
// /boot as a mountpoint.  If there is no explicit boot partition, the root
// partition is returned.
// If neither boot nor root partitions are found, returns -1.
func (pt *PartitionTable) BootPartitionIndex() int {
	// find partition with '/boot' mountpoint and fallback to '/'
	rootIdx := -1
	for idx, part := range pt.Partitions {
		if part.Filesystem == nil {
			continue
		}
		if part.Filesystem.Mountpoint == "/boot" {
			return idx
		} else if part.Filesystem.Mountpoint == "/" {
			rootIdx = idx
		}
	}
	return rootIdx
}

func (pt *PartitionTable) RootPartitionIndex() int {
	rootIdx := -1
	for idx, part := range pt.Partitions {
		if part.Filesystem == nil {
			continue
		}
		if part.Filesystem.Mountpoint == "/" {
			rootIdx = idx
		}
	}
	return rootIdx
}

// Returns the Filesystem instance for a given mountpoint, if it exists.
func (pt *PartitionTable) FindFilesystemForMountpoint(mountpoint string) *Filesystem {
	for _, part := range pt.Partitions {
		if part.Filesystem == nil {
			continue
		}
		if part.Filesystem.Mountpoint == mountpoint {
			return part.Filesystem
		}
	}
	return nil
}

// Returns the Filesystem instance that corresponds to the root
// filesystem, i.e. the filesystem whose mountpoint is '/'.
func (pt *PartitionTable) RootFilesystem() *Filesystem {
	return pt.FindFilesystemForMountpoint("/")
}

// Returns the Filesystem instance that corresponds to the boot
// filesystem, i.e. the filesystem whose mountpoint is '/boot',
// if /boot is on a separate partition, otherwise nil
func (pt *PartitionTable) BootFilesystem() *Filesystem {
	return pt.FindFilesystemForMountpoint("/boot")
}

// dynamically calculate and update the start point
// for each of the existing partitions
// return the updated start point
func (pt *PartitionTable) updatePartitionStartPointOffsets(start uint64) uint64 {
	var rootIdx = -1
	for i := range pt.Partitions {
		partition := &pt.Partitions[i]
		if partition.Filesystem != nil && partition.Filesystem.Mountpoint == "/" {
			rootIdx = i
			continue
		}
		partition.Start = start
		start += partition.Size
	}
	pt.Partitions[rootIdx].Start = start
	return start
}

func (pt *PartitionTable) getPartitionTableSize() uint64 {
	var size uint64
	for _, p := range pt.Partitions {
		size += p.Size
	}
	return size
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

// Converts Filesystem to osbuild.QEMUFilesystem that encodes the same fs.
func (fs *Filesystem) QEMUFilesystem() osbuild.QEMUFilesystem {
	return osbuild.QEMUFilesystem{
		Type:       fs.Type,
		UUID:       fs.UUID,
		Label:      fs.Label,
		Mountpoint: fs.Mountpoint,
	}
}
