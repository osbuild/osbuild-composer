package disk

import (
	"io"
	"math/rand"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
)

const (
	sectorSize            = 512
	BIOSBootPartitionGUID = "21686148-6449-6E6F-744E-656564454649"
	BIOSBootPartitionUUID = "FAC7F1FB-3E8D-4137-A512-961DE09A5549"

	FilesystemDataGUID = "0FC63DAF-8483-4772-8E79-3D69D8477DE4"
	FilesystemDataUUID = "CB07C243-BC44-4717-853E-28852021225B"

	EFISystemPartitionGUID = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
	EFISystemPartitionUUID = "68B2905B-DF3E-4FB3-80FA-49D1E773AA33"
	EFIFilesystemUUID      = "7B77-95E7"

	RootPartitionUUID = "6264D520-3FB9-423F-8AB8-7A0A8E3D3562"
)

func CreatePartitionTable(
	mountpoints []blueprint.FilesystemCustomization,
	imageSize uint64,
	basePartitionTable PartitionTable,
	rng *rand.Rand,
) PartitionTable {

	basePartitionTable.Size = imageSize
	partitions := []Partition{}

	if bootPartition := basePartitionTable.BootPartition(); bootPartition != nil {
		// the boot partition UUID needs to be set since this
		// needs to be randomly generated
		bootPartition.Filesystem.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
	}

	// start point for all of the arches is
	// 2048 sectors.
	var start uint64 = basePartitionTable.updatePartitionStartPointOffsets(2048)

	for _, m := range mountpoints {
		if m.Mountpoint != "/" {
			partitionSize := uint64(m.MinSize) / sectorSize
			partition := createPartition(m.Mountpoint, partitionSize, start, archName, rng)
			partitions = append(partitions, partition)
			start += uint64(m.MinSize / sectorSize)
		}
	}

	// treat the root partition as a special case
	// by setting the size dynamically
	rootPartition := basePartitionTable.RootPartition()
	rootPartition.Start = start
	rootPartition.Size = ((imageSize / sectorSize) - start - 100)
	rootPartition.Filesystem.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()

	basePartitionTable.updateRootPartition(*rootPartition)
	basePartitionTable.Partitions = append(basePartitionTable.Partitions, partitions...)

	return basePartitionTable
}

func createPartition(mountpoint string, size uint64, start uint64, archName string, rng *rand.Rand) Partition {
	filesystem := Filesystem{
		Type:         "xfs",
		UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
		Mountpoint:   mountpoint,
		FSTabOptions: "defaults",
		FSTabFreq:    0,
		FSTabPassNo:  0,
	}
	if archName == distro.Ppc64leArchName || archName == distro.S390xArchName {
		return Partition{
			Start:      start,
			Size:       size,
			Filesystem: &filesystem,
		}
	}
	return Partition{
		Start:      start,
		Size:       size,
		Type:       FilesystemDataGUID,
		UUID:       uuid.Must(newRandomUUIDFromReader(rng)).String(),
		Filesystem: &filesystem,
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
