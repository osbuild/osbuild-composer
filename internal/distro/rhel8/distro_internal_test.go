package rhel8

import (
	"math/rand"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
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

// math/rand is good enough in this case
/* #nosec G404 */
var rng = rand.New(rand.NewSource(0))

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
			assert.True(t, pt.ContainsMountpoint(m.Mountpoint))
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
				assert.True(t, pt.ContainsMountpoint(m.Mountpoint))
			}
		} else {
			require.EqualError(t, err, "unknown arch: "+testEc2ImageType.arch.name)
		}
	}
}
