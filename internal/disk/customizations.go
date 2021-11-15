package disk

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
)

const (
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
	basePartitionTable *PartitionTable,
	rng *rand.Rand,
) (PartitionTable, error) {

	// we are modifying the contents of the base partition table,
	// including the file systems, which are shared among shallow
	// copies of the partition table, so make a copy first
	table := basePartitionTable.Clone()

	for _, m := range mountpoints {
		sectors := table.BytesToSectors(m.MinSize)
		if m.Mountpoint != "/" {
			table.CreateFilesystem(m.Mountpoint, sectors)
		}
	}

	if tableSize := table.getPartitionTableSize(); imageSize < tableSize {
		imageSize = tableSize
	}

	table.Size = imageSize

	// start point for all of the arches is
	// 2048 sectors.
	var start uint64 = table.updatePartitionStartPointOffsets(2048)

	// treat the root partition as a special case
	// by setting the size dynamically
	rootPartition := table.RootPartition()
	rootPartition.Size = (table.BytesToSectors(imageSize) - start - 100)

	// Generate new UUIDs for filesystems and partitions
	table.GenerateUUIDs(rng)

	return *table, nil
}
