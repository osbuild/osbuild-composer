package disk_test

import (
	"math/rand"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/stretchr/testify/assert"
)

func TestDisk_DynamicallyResizePartitionTable(t *testing.T) {
	mountpoints := []blueprint.FilesystemCustomization{
		{
			MinSize:    "2147483648",
			Mountpoint: "/usr",
		},
	}
	pt := disk.PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []disk.Partition{
			{
				Size:     2048,
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
				Type: disk.FilesystemDataGUID,
				UUID: disk.RootPartitionUUID,
				Filesystem: &disk.Filesystem{
					Type:         "xfs",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
		},
	}
	var expectedSize uint64 = 2147483648
	rng := rand.New(rand.NewSource(0))
	pt = disk.CreatePartitionTable(mountpoints, 1024, pt, rng)
	assert.GreaterOrEqual(t, expectedSize, pt.Size)
}
