package rhel85

import (
	"math/rand"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testBasicImageType = imageType{
	name:                "test",
	basePartitionTables: defaultBasePartitionTables,
}

var testEc2ImageType = imageType{
	name:                "test_ec2",
	basePartitionTables: ec2BasePartitionTables,
}

var mountpoints = []blueprint.FilesystemCustomization{
	{
		MinSize:    1024,
		Mountpoint: "/usr",
	},
}

var rng = rand.New(rand.NewSource(0))

func containsMountpoint(expected []disk.Partition, mountpoint string) bool {
	for _, p := range expected {
		if p.Filesystem == nil {
			continue
		}
		if p.Filesystem.Mountpoint == mountpoint {
			return true
		}
	}
	return false
}

func TestDistro_UnsupportedArch(t *testing.T) {
	testBasicImageType.arch = &architecture{
		name: "unsupported_arch",
	}
	_, err := testBasicImageType.getPartitionTable(mountpoints, distro.ImageOptions{}, rng)
	require.EqualError(t, err, "unknown arch: "+testBasicImageType.arch.name)
}

func TestDistro_DefaultPartitionTables(t *testing.T) {
	rhel8distro := New()
	for _, archName := range rhel8distro.ListArches() {
		testBasicImageType.arch = &architecture{
			name: archName,
		}
		pt, err := testBasicImageType.getPartitionTable(mountpoints, distro.ImageOptions{}, rng)
		require.Nil(t, err)
		for _, m := range mountpoints {
			contains := containsMountpoint(pt.Partitions, m.Mountpoint)
			assert.True(t, contains)
		}
	}
}

func TestDistro_Ec2PartitionTables(t *testing.T) {
	rhel8distro := New()
	for _, archName := range rhel8distro.ListArches() {
		testEc2ImageType.arch = &architecture{
			name: archName,
		}
		pt, err := testEc2ImageType.getPartitionTable(mountpoints, distro.ImageOptions{}, rng)
		if _, exists := testEc2ImageType.basePartitionTables[archName]; exists {
			require.Nil(t, err)
			for _, m := range mountpoints {
				contains := containsMountpoint(pt.Partitions, m.Mountpoint)
				assert.True(t, contains)
			}
		} else {
			require.EqualError(t, err, "unknown arch: "+testEc2ImageType.arch.name)
		}
	}
}
