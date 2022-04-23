package disk

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/stretchr/testify/assert"
)

func TestDisk_AlignUp(t *testing.T) {

	pt := PartitionTable{}
	firstAligned := DefaultGrainBytes

	tests := []struct {
		size uint64
		want uint64
	}{
		{0, 0},
		{1, firstAligned},
		{firstAligned - 1, firstAligned},
		{firstAligned, firstAligned}, // grain is already aligned => no change
		{firstAligned / 2, firstAligned},
		{firstAligned + 1, firstAligned * 2},
	}

	for _, tt := range tests {
		got := pt.AlignUp(tt.size)
		assert.Equal(t, tt.want, got, "Expected %d, got %d", tt.want, got)
	}
}

func TestDisk_DynamicallyResizePartitionTable(t *testing.T) {
	mountpoints := []blueprint.FilesystemCustomization{
		{
			MinSize:    2147483648,
			Mountpoint: "/usr",
		},
	}
	pt := PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []Partition{
			{
				Size:     2048,
				Bootable: true,
				Type:     BIOSBootPartitionGUID,
				UUID:     BIOSBootPartitionUUID,
			},
			{
				Type: FilesystemDataGUID,
				UUID: RootPartitionUUID,
				Payload: &Filesystem{
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
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(0))
	newpt, err := NewPartitionTable(&pt, mountpoints, 1024, false, rng)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, newpt.Size, expectedSize)
}

var testPartitionTables = map[string]PartitionTable{
	"plain": {
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []Partition{
			{
				Size:     1048576, // 1MB
				Bootable: true,
				Type:     BIOSBootPartitionGUID,
				UUID:     BIOSBootPartitionUUID,
			},
			{
				Size: 209715200, // 200 MB
				Type: EFISystemPartitionGUID,
				UUID: EFISystemPartitionUUID,
				Payload: &Filesystem{
					Type:         "vfat",
					UUID:         EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					Label:        "EFI-SYSTEM",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 1024000, // 500 MB
				Type: FilesystemDataGUID,
				UUID: FilesystemDataUUID,
				Payload: &Filesystem{
					Type:         "xfs",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
			{
				Type: FilesystemDataGUID,
				UUID: RootPartitionUUID,
				Payload: &Filesystem{
					Type:         "xfs",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
		},
	},

	"plain-noboot": {
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []Partition{
			{
				Size:     1048576, // 1MB
				Bootable: true,
				Type:     BIOSBootPartitionGUID,
				UUID:     BIOSBootPartitionUUID,
			},
			{
				Size: 209715200, // 200 MB
				Type: EFISystemPartitionGUID,
				UUID: EFISystemPartitionUUID,
				Payload: &Filesystem{
					Type:         "vfat",
					UUID:         EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					Label:        "EFI-SYSTEM",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Type: FilesystemDataGUID,
				UUID: RootPartitionUUID,
				Payload: &Filesystem{
					Type:         "xfs",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
		},
	},

	"luks": {
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []Partition{
			{
				Size:     1048576, // 1MB
				Bootable: true,
				Type:     BIOSBootPartitionGUID,
				UUID:     BIOSBootPartitionUUID,
			},
			{
				Size: 209715200, // 200 MB
				Type: EFISystemPartitionGUID,
				UUID: EFISystemPartitionUUID,
				Payload: &Filesystem{
					Type:         "vfat",
					UUID:         EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					Label:        "EFI-SYSTEM",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 1024000, // 500 MB
				Type: FilesystemDataGUID,
				UUID: FilesystemDataUUID,
				Payload: &Filesystem{
					Type:         "xfs",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
			{
				Type: FilesystemDataGUID,
				UUID: RootPartitionUUID,
				Payload: &LUKSContainer{
					UUID:  "",
					Label: "crypt_root",
					Payload: &Filesystem{
						Type:         "xfs",
						Label:        "root",
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		},
	},
	"luks+lvm": {
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []Partition{
			{
				Size:     1048576, // 1MB
				Bootable: true,
				Type:     BIOSBootPartitionGUID,
				UUID:     BIOSBootPartitionUUID,
			},
			{
				Size: 209715200, // 200 MB
				Type: EFISystemPartitionGUID,
				UUID: EFISystemPartitionUUID,
				Payload: &Filesystem{
					Type:         "vfat",
					UUID:         EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					Label:        "EFI-SYSTEM",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 1024000, // 500 MB
				Type: FilesystemDataGUID,
				UUID: FilesystemDataUUID,
				Payload: &Filesystem{
					Type:         "xfs",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
			{
				Type: FilesystemDataGUID,
				UUID: RootPartitionUUID,
				Size: 5 * 1024 * 1024 * 1024,
				Payload: &LUKSContainer{
					UUID: "",
					Payload: &LVMVolumeGroup{
						Name:        "",
						Description: "",
						LogicalVolumes: []LVMLogicalVolume{
							{
								Size: 2 * 1024 * 1024 * 1024,
								Payload: &Filesystem{
									Type:         "xfs",
									Label:        "root",
									Mountpoint:   "/",
									FSTabOptions: "defaults",
									FSTabFreq:    0,
									FSTabPassNo:  0,
								},
							},
							{
								Size: 2 * 1024 * 1024 * 1024,
								Payload: &Filesystem{
									Type:         "xfs",
									Label:        "root",
									Mountpoint:   "/home",
									FSTabOptions: "defaults",
									FSTabFreq:    0,
									FSTabPassNo:  0,
								},
							},
						},
					},
				},
			},
		},
	},
	"btrfs": {
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []Partition{
			{
				Size:     1048576, // 1MB
				Bootable: true,
				Type:     BIOSBootPartitionGUID,
				UUID:     BIOSBootPartitionUUID,
			},
			{
				Size: 209715200, // 200 MB
				Type: EFISystemPartitionGUID,
				UUID: EFISystemPartitionUUID,
				Payload: &Filesystem{
					Type:         "vfat",
					UUID:         EFIFilesystemUUID,
					Mountpoint:   "/boot/efi",
					Label:        "EFI-SYSTEM",
					FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
					FSTabFreq:    0,
					FSTabPassNo:  2,
				},
			},
			{
				Size: 1024000, // 500 MB
				Type: FilesystemDataGUID,
				UUID: FilesystemDataUUID,
				Payload: &Filesystem{
					Type:         "xfs",
					Mountpoint:   "/boot",
					Label:        "boot",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
			{
				Type: FilesystemDataGUID,
				UUID: RootPartitionUUID,
				Size: 10 * 1024 * 1024 * 1024,
				Payload: &Btrfs{
					UUID:       "",
					Label:      "",
					Mountpoint: "",
					Subvolumes: []BtrfsSubvolume{
						{
							Size:       0,
							Mountpoint: "/",
							GroupID:    0,
						},
						{
							Size:       5 * 1024 * 1024 * 1024,
							Mountpoint: "/var",
							GroupID:    0,
						},
					},
				},
			},
		},
	},
}

var bp = []blueprint.FilesystemCustomization{
	{
		Mountpoint: "/",
		MinSize:    10 * 1024 * 1024 * 1024,
	},
	{
		Mountpoint: "/home",
		MinSize:    20 * 1024 * 1024 * 1024,
	},
	{
		Mountpoint: "/opt",
		MinSize:    7 * 1024 * 1024 * 1024,
	},
}

var bp2 = []blueprint.FilesystemCustomization{
	{
		Mountpoint: "/opt",
		MinSize:    7 * 1024 * 1024 * 1024,
	},
}

func TestDisk_ForEachEntity(t *testing.T) {

	count := 0

	plain := testPartitionTables["plain"]
	err := plain.ForEachEntity(func(e Entity, path []Entity) error {
		assert.NotNil(t, e)
		assert.NotNil(t, path)

		count += 1
		return nil
	})

	assert.NoError(t, err)

	// PartitionTable, 4 partitions, 3 filesystems -> 8 entities
	assert.Equal(t, 8, count)
}

func TestCreatePartitionTable(t *testing.T) {
	assert := assert.New(t)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(13))
	for name := range testPartitionTables {
		pt := testPartitionTables[name]
		mpt, err := NewPartitionTable(&pt, bp, uint64(13*1024*1024), false, rng)
		assert.NoError(err, "Partition table generation failed: %s (%s)", name, err)
		assert.NotNil(mpt, "Partition table generation failed: %s (nil partition table)", name)
		assert.Greater(mpt.GetSize(), uint64(37*1024*1024*1024))

		assert.NotNil(mpt.Type, "Partition table generation failed: %s (nil partition table type)", name)

		mnt := pt.FindMountable("/")
		assert.NotNil(mnt, "Partition table '%s': failed to find root mountable", name)
	}
}

func TestCreatePartitionTableLVMify(t *testing.T) {
	assert := assert.New(t)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(13))
	blueprints := [][]blueprint.FilesystemCustomization{bp, bp2}
	for _, tbp := range blueprints {
		for name := range testPartitionTables {
			pt := testPartitionTables[name]

			if name == "btrfs" || name == "luks" {
				assert.Panics(func() {
					_, _ = NewPartitionTable(&pt, tbp, uint64(13*1024*1024), true, rng)
				})
				continue
			}

			mpt, err := NewPartitionTable(&pt, tbp, uint64(13*1024*1024), true, rng)
			assert.NoError(err, "Partition table generation failed: %s (%s)", name, err)

			rootPath := entityPath(mpt, "/")
			if rootPath == nil {
				panic("no root mountpoint for PartitionTable")
			}

			bootPath := entityPath(mpt, "/boot")
			if bootPath == nil {
				panic("no boot mountpoint for PartitionTable")
			}

			parent := rootPath[1]
			_, ok := parent.(*LVMLogicalVolume)
			assert.True(ok, "Partition table '%s': root's parent (%q) is not an LVM logical volume", name, parent)
		}
	}
}

func collectEntities(pt *PartitionTable) []Entity {
	entities := make([]Entity, 0)
	collector := func(ent Entity, path []Entity) error {
		entities = append(entities, ent)
		return nil
	}
	_ = pt.ForEachEntity(collector)
	return entities
}

func TestClone(t *testing.T) {
	for name := range testPartitionTables {
		basePT := testPartitionTables[name]
		baseEntities := collectEntities(&basePT)

		clonePT := basePT.Clone().(*PartitionTable)
		cloneEntities := collectEntities(clonePT)

		for idx := range baseEntities {
			for jdx := range cloneEntities {
				if fmt.Sprintf("%p", baseEntities[idx]) == fmt.Sprintf("%p", cloneEntities[jdx]) {
					t.Fatalf("found reference to same entity %#v in list of clones for partition table %q", baseEntities[idx], name)
				}
			}
		}
	}
}
