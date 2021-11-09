package disk

import (
	"encoding/hex"
	"io"
	"math/rand"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
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

	if bootPartition := basePartitionTable.BootPartition(); bootPartition != nil {
		// the boot partition UUID needs to be set since this
		// needs to be randomly generated
		bootPartition.Filesystem.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()
	}

	for _, m := range mountpoints {
		if m.Mountpoint != "/" {
			partitionSize := m.MinSize / sectorSize
			partition := basePartitionTable.createPartition(m.Mountpoint, partitionSize, rng)
			basePartitionTable.Partitions = append(basePartitionTable.Partitions, partition)
		}
	}

	if tableSize := basePartitionTable.getPartitionTableSize(); imageSize < tableSize {
		imageSize = tableSize
	}

	basePartitionTable.Size = imageSize

	// start point for all of the arches is
	// 2048 sectors.
	var start uint64 = basePartitionTable.updatePartitionStartPointOffsets(2048)

	// treat the root partition as a special case
	// by setting the size dynamically
	rootPartition := basePartitionTable.RootPartition()
	rootPartition.Size = ((imageSize / sectorSize) - start - 100)
	rootPartition.Filesystem.UUID = uuid.Must(newRandomUUIDFromReader(rng)).String()

	return basePartitionTable
}

func (pt *PartitionTable) createPartition(mountpoint string, size uint64, rng *rand.Rand) Partition {
	filesystem := Filesystem{
		Type:         "xfs",
		UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
		Mountpoint:   mountpoint,
		FSTabOptions: "defaults",
		FSTabFreq:    0,
		FSTabPassNo:  0,
	}
	if pt.Type != "gpt" {
		return Partition{
			Size:       size,
			Filesystem: &filesystem,
		}
	}
	return Partition{
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

// NewRandomVolIDFromReader creates a random 32 bit hex string to use as a
// volume ID for FAT filesystems
func NewRandomVolIDFromReader(r io.Reader) (string, error) {
	volid := make([]byte, 4)
	_, err := r.Read(volid)
	return hex.EncodeToString(volid), err
}
