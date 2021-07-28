package rhel85

import (
	"io"
	"math/rand"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
)

const sectorSize = 512

func createPartitionTable(
	mountpoints []blueprint.FilesystemCustomization,
	imageOptions distro.ImageOptions,
	arch distro.Arch,
	rng *rand.Rand,
) disk.PartitionTable {
	if arch.Name() == "x86_64" {
		partitionTable := disk.PartitionTable{
			Size: imageOptions.Size,
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: "gpt",
			Partitions: []disk.Partition{
				{
					Bootable: true,
					Size:     2048,
					Start:    2048,
					Type:     "21686148-6449-6E6F-744E-656564454649",
					UUID:     "FAC7F1FB-3E8D-4137-A512-961DE09A5549",
				},
				{
					Start: 4096,
					Size:  204800,
					Type:  "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
					UUID:  "68B2905B-DF3E-4FB3-80FA-49D1E773AA33",
					Filesystem: &disk.Filesystem{
						Type:         "vfat",
						UUID:         "7B77-95E7",
						Mountpoint:   "/boot/efi",
						FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
						FSTabFreq:    0,
						FSTabPassNo:  2,
					},
				},
			},
		}
		start := uint64(4096 + 204800)
		partitions := []disk.Partition{}
		for _, m := range mountpoints {
			if m.Mountpoint != "/" {
				partition := createPartition(m, start, arch.Name(), rng)
				partitions = append(partitions, partition)
				start += uint64(m.MinSize / sectorSize)
			}
		}
		// treat the root partition as a special case
		// by setting it last and setting the size
		// dynamically
		rootPartition := disk.Partition{
			Start: start,
			Size:  imageOptions.Size/sectorSize - start - 100,
			Type:  "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
			UUID:  "6264D520-3FB9-423F-8AB8-7A0A8E3D3562",
			Filesystem: &disk.Filesystem{
				Type:         "xfs",
				UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
				Label:        "root",
				Mountpoint:   "/",
				FSTabOptions: "defaults",
				FSTabFreq:    0,
				FSTabPassNo:  0,
			},
		}
		partitions = append(partitions, rootPartition)
		partitionTable.Partitions = append(partitionTable.Partitions, partitions...)
		return partitionTable
	} else if arch.Name() == "aarch64" {
		partitionTable := disk.PartitionTable{
			Size: imageOptions.Size,
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: "gpt",
			Partitions: []disk.Partition{
				{
					Start: 2048,
					Size:  204800,
					Type:  "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
					UUID:  "68B2905B-DF3E-4FB3-80FA-49D1E773AA33",
					Filesystem: &disk.Filesystem{
						Type:         "vfat",
						UUID:         "7B77-95E7",
						Mountpoint:   "/boot/efi",
						FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
						FSTabFreq:    0,
						FSTabPassNo:  2,
					},
				},
			},
		}
		start := uint64(2048 + 204800)
		partitions := []disk.Partition{}
		for _, m := range mountpoints {
			if m.Mountpoint != "/" {
				partition := createPartition(m, start, arch.Name(), rng)
				partitions = append(partitions, partition)
				start += uint64(m.MinSize / sectorSize)
			}
		}
		rootPartition := disk.Partition{
			Start: start,
			Size:  imageOptions.Size/sectorSize - start - 100,
			Type:  "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
			UUID:  "6264D520-3FB9-423F-8AB8-7A0A8E3D3562",
			Filesystem: &disk.Filesystem{
				Type:         "xfs",
				UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
				Label:        "root",
				Mountpoint:   "/",
				FSTabOptions: "defaults",
				FSTabFreq:    0,
				FSTabPassNo:  0,
			},
		}
		partitions = append(partitions, rootPartition)
		partitionTable.Partitions = append(partitionTable.Partitions, partitions...)
		return partitionTable
	} else if arch.Name() == "ppc64le" {
		partitionTable := disk.PartitionTable{
			Size: imageOptions.Size,
			UUID: "0x14fc63d2",
			Type: "dos",
			Partitions: []disk.Partition{
				{
					Start:    2048,
					Size:     8192,
					Type:     "41",
					Bootable: true,
				},
			},
		}
		start := uint64(8192)
		partitions := []disk.Partition{}
		for _, m := range mountpoints {
			if m.Mountpoint != "/" {
				partition := createPartition(m, start, arch.Name(), rng)
				partitions = append(partitions, partition)
				start += uint64(m.MinSize / sectorSize)
			}
		}
		rootPartition := disk.Partition{
			Start: start,
			Size:  imageOptions.Size/sectorSize - start - 100,
			Filesystem: &disk.Filesystem{
				Type:         "xfs",
				UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
				Mountpoint:   "/",
				FSTabOptions: "defaults",
				FSTabFreq:    0,
				FSTabPassNo:  0,
			},
		}
		partitions = append(partitions, rootPartition)
		partitionTable.Partitions = append(partitionTable.Partitions, partitions...)
		return partitionTable
	} else if arch.Name() == "s390x" {
		partitionTable := disk.PartitionTable{
			Size:       imageOptions.Size,
			UUID:       "0x14fc63d2",
			Type:       "dos",
			Partitions: []disk.Partition{},
		}
		start := uint64(2048)
		partitions := []disk.Partition{}
		for _, m := range mountpoints {
			if m.Mountpoint != "/" {
				partition := createPartition(m, start, arch.Name(), rng)
				partitions = append(partitions, partition)
				start += uint64(m.MinSize / sectorSize)
			}
		}
		rootPartition := disk.Partition{
			Start:    start,
			Size:     imageOptions.Size/sectorSize - start - 100,
			Bootable: true,
			Filesystem: &disk.Filesystem{
				Type:         "xfs",
				UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
				Mountpoint:   "/",
				FSTabOptions: "defaults",
				FSTabFreq:    0,
				FSTabPassNo:  0,
			},
		}
		partitions = append(partitions, rootPartition)
		partitionTable.Partitions = append(partitionTable.Partitions, partitions...)
		return partitionTable
	}

	panic("unknown arch: " + arch.Name())
}

func ec2PartitionTable(imageOptions distro.ImageOptions, arch distro.Arch, rng *rand.Rand) disk.PartitionTable {
	var sectorSize uint64 = 512
	if arch.Name() == "x86_64" {
		return disk.PartitionTable{
			Size: imageOptions.Size,
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: "gpt",
			Partitions: []disk.Partition{
				{
					Bootable: true,
					Size:     2048,
					Start:    2048,
					Type:     "21686148-6449-6E6F-744E-656564454649",
					UUID:     "FAC7F1FB-3E8D-4137-A512-961DE09A5549",
				},
				{
					Start: 4096,
					Size:  imageOptions.Size/sectorSize - 4096 - 100,
					Type:  "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
					UUID:  "6264D520-3FB9-423F-8AB8-7A0A8E3D3562",
					Filesystem: &disk.Filesystem{
						Type:         "xfs",
						UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
						Label:        "root",
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		}
	} else if arch.Name() == "aarch64" {
		return disk.PartitionTable{
			Size: imageOptions.Size,
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: "gpt",
			Partitions: []disk.Partition{
				{
					Start: 2048,
					Size:  409600,
					Type:  "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
					UUID:  "68B2905B-DF3E-4FB3-80FA-49D1E773AA33",
					Filesystem: &disk.Filesystem{
						Type:         "vfat",
						UUID:         "7B77-95E7",
						Mountpoint:   "/boot/efi",
						FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
						FSTabFreq:    0,
						FSTabPassNo:  2,
					},
				},
				{
					Start: 411648,
					Size:  1048576,
					Type:  "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
					UUID:  "CB07C243-BC44-4717-853E-28852021225B",
					Filesystem: &disk.Filesystem{
						Type:         "xfs",
						UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
						Mountpoint:   "/boot",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
				{
					Start: 1460224,
					Size:  imageOptions.Size/sectorSize - 1460224 - 100,
					Type:  "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
					UUID:  "6264D520-3FB9-423F-8AB8-7A0A8E3D3562",
					Filesystem: &disk.Filesystem{
						Type:         "xfs",
						UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
						Label:        "root",
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		}
	}

	panic("unsupported EC2 arch: " + arch.Name())
}

func createPartition(
	m blueprint.FilesystemCustomization,
	start uint64,
	archName string,
	rng *rand.Rand,
) disk.Partition {
	if archName == "x86_64" || archName == "aarch64" {
		return disk.Partition{
			Start: start,
			Size:  uint64(m.MinSize) / sectorSize,
			Type:  "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
			UUID:  uuid.Must(newRandomUUIDFromReader(rng)).String(),
			Filesystem: &disk.Filesystem{
				Type:         "xfs",
				UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
				Mountpoint:   m.Mountpoint,
				FSTabOptions: "defaults",
				FSTabFreq:    0,
				FSTabPassNo:  0,
			},
		}
	}
	return disk.Partition{
		Start: start,
		Size:  uint64(m.MinSize),
		Filesystem: &disk.Filesystem{
			Type:         "xfs",
			UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
			Mountpoint:   m.Mountpoint,
			FSTabOptions: "defaults",
			FSTabFreq:    0,
			FSTabPassNo:  0,
		},
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
