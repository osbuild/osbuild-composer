package rhel85

import (
	"io"
	"math/rand"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
)

const (
	sectorSize = 512

	biosBootPartitionType = "21686148-6449-6E6F-744E-656564454649"
	biosBootPartitionUUID = "FAC7F1FB-3E8D-4137-A512-961DE09A5549"

	bootPartitionType = "0FC63DAF-8483-4772-8E79-3D69D8477DE4"
	bootPartitionUUID = "CB07C243-BC44-4717-853E-28852021225B"

	bootEFIPartitionType  = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
	bootEFIPartitionUUID  = "68B2905B-DF3E-4FB3-80FA-49D1E773AA33"
	bootEFIFilesystemUUID = "7B77-95E7"

	defaultPartitionType = "0FC63DAF-8483-4772-8E79-3D69D8477DE4"
	rootPartitionUUID    = "6264D520-3FB9-423F-8AB8-7A0A8E3D3562"
)

var x86PartitionTable = disk.PartitionTable{
	UUID:       "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
	Type:       "gpt",
	Partitions: []disk.Partition{},
}

var aarch64PartitionTable = disk.PartitionTable{
	UUID:       "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
	Type:       "gpt",
	Partitions: []disk.Partition{},
}

var ppc64lePartitionTable = disk.PartitionTable{
	UUID:       "0x14fc63d2",
	Type:       "dos",
	Partitions: []disk.Partition{},
}

var s390xPartitionTable = disk.PartitionTable{
	UUID:       "0x14fc63d2",
	Type:       "dos",
	Partitions: []disk.Partition{},
}

var validArches = map[string]disk.PartitionTable{
	x86_64ArchName:  x86PartitionTable,
	aarch64ArchName: aarch64PartitionTable,
	ppc64leArchName: ppc64lePartitionTable,
	s390xArchName:   s390xPartitionTable,
}

var ec2ValidArches = map[string]disk.PartitionTable{
	x86_64ArchName:  x86PartitionTable,
	aarch64ArchName: aarch64PartitionTable,
}

func createPartitionTable(
	mountpoints []blueprint.FilesystemCustomization,
	imageOptions distro.ImageOptions,
	arch distro.Arch,
	rng *rand.Rand,
) disk.PartitionTable {
	archName := arch.Name()
	partitionTable, ok := validArches[archName]

	if !ok {
		panic("unknown arch: " + archName)
	}

	partitionTable.Size = imageOptions.Size
	partitions := []disk.Partition{}
	var start uint64

	if archName == x86_64ArchName {
		biosBootPartition := disk.Partition{
			Bootable: true,
			Size:     2048,
			Start:    2048,
			Type:     biosBootPartitionType,
			UUID:     biosBootPartitionUUID,
		}
		partitions = append(partitions, biosBootPartition)
		bootEFIPartition := createPartition("/boot/efi", 204800, 4096, archName, rng)
		partitions = append(partitions, bootEFIPartition)
		start = uint64(4096 + 204800)
	} else if archName == aarch64ArchName {
		bootEFIPartition := createPartition("/boot/efi", 204800, 2048, archName, rng)
		partitions = append(partitions, bootEFIPartition)
		start = uint64(2048 + 204800)
	} else if archName == ppc64leArchName {
		biosBootPartition := disk.Partition{
			Start:    2048,
			Size:     8192,
			Type:     "41",
			Bootable: true,
		}
		partitions = append(partitions, biosBootPartition)
		start = uint64(10240)
	} else if archName == s390xArchName {
		start = uint64(2048)
	}

	for _, m := range mountpoints {
		if m.Mountpoint != "/" {
			partitionSize := uint64(m.MinSize) / sectorSize
			partition := createPartition(m.Mountpoint, partitionSize, start, archName, rng)
			partitions = append(partitions, partition)
			start += uint64(m.MinSize / sectorSize)
		}
	}

	// treat the root partition as a special case
	// by setting it last and setting the size
	// dynamically
	rootSize := (imageOptions.Size / sectorSize) - start - 100
	rootPartition := createPartition("/", rootSize, start, archName, rng)
	partitions = append(partitions, rootPartition)

	partitionTable.Partitions = append(partitionTable.Partitions, partitions...)

	return partitionTable
}

func ec2PartitionTable(imageOptions distro.ImageOptions, arch distro.Arch, rng *rand.Rand) disk.PartitionTable {
	archName := arch.Name()

	partitionTable, ok := ec2ValidArches[archName]

	if !ok {
		panic("unsupported EC2 arch: " + archName)
	}

	partitionTable.Size = imageOptions.Size
	partitions := []disk.Partition{}
	var start uint64

	if archName == x86_64ArchName {
		biosBootPartition := disk.Partition{
			Bootable: true,
			Size:     2048,
			Start:    2048,
			Type:     biosBootPartitionType,
			UUID:     biosBootPartitionUUID,
		}
		partitions = append(partitions, biosBootPartition)
		start = 4096
	} else if archName == aarch64ArchName {
		bootEFIParition := createPartition("/boot/efi", 409600, 2048, archName, rng)
		partitions = append(partitions, bootEFIParition)
		bootPartition := createPartition("/boot", 1048576, 411648, archName, rng)
		partitions = append(partitions, bootPartition)
		start = 1460224
	}
	rootSize := (imageOptions.Size / sectorSize) - start - 100
	rootPartition := createPartition("/", rootSize, start, archName, rng)
	partitions = append(partitions, rootPartition)
	partitionTable.Partitions = append(partitionTable.Partitions, partitions...)
	return partitionTable
}

func createPartition(mountpoint string, size uint64, start uint64, archName string, rng *rand.Rand) disk.Partition {
	var filesystem disk.Filesystem
	// /boot/efi mountpoint is a special case
	// return early
	if mountpoint == "/boot/efi" {
		filesystem = createFilesystemDisk(mountpoint, bootEFIFilesystemUUID)
		return disk.Partition{
			Start:      start,
			Size:       size,
			Type:       bootEFIPartitionType,
			UUID:       bootEFIPartitionUUID,
			Filesystem: &filesystem,
		}
	}
	partition := disk.Partition{
		Start:      start,
		Size:       size,
		Filesystem: &filesystem,
	}
	diskUUID := uuid.Must(newRandomUUIDFromReader(rng)).String()
	filesystem = createFilesystemDisk(mountpoint, diskUUID)
	if mountpoint == "/boot" {
		partition.Type = bootPartitionType
		partition.UUID = bootPartitionUUID
		return partition
	}
	if archName == x86_64ArchName || archName == aarch64ArchName {
		if mountpoint == "/" {
			// set Label for root mountpoint
			filesystem.Label = "root"
			partition.Type = defaultPartitionType
			partition.UUID = rootPartitionUUID
			return partition
		}
		partition.Type = defaultPartitionType
		partition.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
		return partition
	}
	if mountpoint == "/" && archName == s390xArchName {
		partition.Bootable = true
	}
	return partition
}

func createFilesystemDisk(mountpoint string, uuid string) disk.Filesystem {
	if mountpoint == "/boot/efi" {
		return disk.Filesystem{
			Type:         "vfat",
			UUID:         uuid,
			Mountpoint:   mountpoint,
			FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
			FSTabFreq:    0,
			FSTabPassNo:  2,
		}
	}
	return disk.Filesystem{
		Type:         "xfs",
		UUID:         uuid,
		Mountpoint:   mountpoint,
		FSTabOptions: "defaults",
		FSTabFreq:    0,
		FSTabPassNo:  0,
	}
}

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
