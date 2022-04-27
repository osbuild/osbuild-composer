package disk

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/stretchr/testify/assert"
)

const (
	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB
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
			MinSize:    2 * GiB,
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
	var expectedSize uint64 = 2 * GiB
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
				Size:     1 * MiB,
				Bootable: true,
				Type:     BIOSBootPartitionGUID,
				UUID:     BIOSBootPartitionUUID,
			},
			{
				Size: 200 * MiB,
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
				Size: 500 * MiB,
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
				Size:     1 * MiB,
				Bootable: true,
				Type:     BIOSBootPartitionGUID,
				UUID:     BIOSBootPartitionUUID,
			},
			{
				Size: 200 * MiB,
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
				Size:     1 * MiB,
				Bootable: true,
				Type:     BIOSBootPartitionGUID,
				UUID:     BIOSBootPartitionUUID,
			},
			{
				Size: 200 * MiB,
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
				Size: 500 * MiB,
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
				Size:     1 * MiB,
				Bootable: true,
				Type:     BIOSBootPartitionGUID,
				UUID:     BIOSBootPartitionUUID,
			},
			{
				Size: 200 * MiB,
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
				Size: 500 * MiB,
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
				Size: 5 * GiB,
				Payload: &LUKSContainer{
					UUID: "",
					Payload: &LVMVolumeGroup{
						Name:        "",
						Description: "",
						LogicalVolumes: []LVMLogicalVolume{
							{
								Size: 2 * GiB,
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
								Size: 2 * GiB,
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
				Size:     1 * MiB,
				Bootable: true,
				Type:     BIOSBootPartitionGUID,
				UUID:     BIOSBootPartitionUUID,
			},
			{
				Size: 200 * MiB,
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
				Size: 500 * MiB,
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
				Size: 10 * GiB,
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
							Size:       5 * GiB,
							Mountpoint: "/var",
							GroupID:    0,
						},
					},
				},
			},
		},
	},
}

var testBlueprints = map[string][]blueprint.FilesystemCustomization{
	"bp1": {
		{
			Mountpoint: "/",
			MinSize:    10 * GiB,
		},
		{
			Mountpoint: "/home",
			MinSize:    20 * GiB,
		},
		{
			Mountpoint: "/opt",
			MinSize:    7 * GiB,
		},
	},
	"bp2": {
		{
			Mountpoint: "/opt",
			MinSize:    7 * GiB,
		},
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
		mpt, err := NewPartitionTable(&pt, testBlueprints["bp1"], uint64(13*MiB), false, rng)
		assert.NoError(err, "Partition table generation failed: %s (%s)", name, err)
		assert.NotNil(mpt, "Partition table generation failed: %s (nil partition table)", name)
		assert.Greater(mpt.GetSize(), uint64(37*GiB))

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
	for _, tbp := range testBlueprints {
		for name := range testPartitionTables {
			pt := testPartitionTables[name]

			if name == "btrfs" || name == "luks" {
				assert.Panics(func() {
					_, _ = NewPartitionTable(&pt, tbp, uint64(13*MiB), true, rng)
				})
				continue
			}

			mpt, err := NewPartitionTable(&pt, tbp, uint64(13*MiB), true, rng)
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

func TestFindDirectoryPartition(t *testing.T) {
	assert := assert.New(t)
	usr := Partition{
		Type: FilesystemDataGUID,
		UUID: RootPartitionUUID,
		Payload: &Filesystem{
			Type:         "xfs",
			Label:        "root",
			Mountpoint:   "/usr",
			FSTabOptions: "defaults",
			FSTabFreq:    0,
			FSTabPassNo:  0,
		},
	}

	{
		pt := testPartitionTables["plain"]
		assert.Equal("/", pt.findDirectoryEntityPath("/opt")[0].(Mountable).GetMountpoint())
		assert.Equal("/boot/efi", pt.findDirectoryEntityPath("/boot/efi/Linux")[0].(Mountable).GetMountpoint())
		assert.Equal("/boot", pt.findDirectoryEntityPath("/boot/loader")[0].(Mountable).GetMountpoint())
		assert.Equal("/boot", pt.findDirectoryEntityPath("/boot")[0].(Mountable).GetMountpoint())

		ptMod := pt.Clone().(*PartitionTable)
		ptMod.Partitions = append(ptMod.Partitions, usr)
		assert.Equal("/", ptMod.findDirectoryEntityPath("/opt")[0].(Mountable).GetMountpoint())
		assert.Equal("/usr", ptMod.findDirectoryEntityPath("/usr")[0].(Mountable).GetMountpoint())
		assert.Equal("/usr", ptMod.findDirectoryEntityPath("/usr/bin")[0].(Mountable).GetMountpoint())

		// invalid dir should return nil
		assert.Nil(pt.findDirectoryEntityPath("invalid"))
	}

	{
		pt := testPartitionTables["plain-noboot"]
		assert.Equal("/", pt.findDirectoryEntityPath("/opt")[0].(Mountable).GetMountpoint())
		assert.Equal("/", pt.findDirectoryEntityPath("/boot")[0].(Mountable).GetMountpoint())
		assert.Equal("/", pt.findDirectoryEntityPath("/boot/loader")[0].(Mountable).GetMountpoint())

		ptMod := pt.Clone().(*PartitionTable)
		ptMod.Partitions = append(ptMod.Partitions, usr)
		assert.Equal("/", ptMod.findDirectoryEntityPath("/opt")[0].(Mountable).GetMountpoint())
		assert.Equal("/usr", ptMod.findDirectoryEntityPath("/usr")[0].(Mountable).GetMountpoint())
		assert.Equal("/usr", ptMod.findDirectoryEntityPath("/usr/bin")[0].(Mountable).GetMountpoint())

		// invalid dir should return nil
		assert.Nil(pt.findDirectoryEntityPath("invalid"))
	}

	{
		pt := testPartitionTables["luks"]
		assert.Equal("/", pt.findDirectoryEntityPath("/opt")[0].(Mountable).GetMountpoint())
		assert.Equal("/boot", pt.findDirectoryEntityPath("/boot")[0].(Mountable).GetMountpoint())
		assert.Equal("/boot", pt.findDirectoryEntityPath("/boot/loader")[0].(Mountable).GetMountpoint())

		ptMod := pt.Clone().(*PartitionTable)
		ptMod.Partitions = append(ptMod.Partitions, usr)
		assert.Equal("/", ptMod.findDirectoryEntityPath("/opt")[0].(Mountable).GetMountpoint())
		assert.Equal("/usr", ptMod.findDirectoryEntityPath("/usr")[0].(Mountable).GetMountpoint())
		assert.Equal("/usr", ptMod.findDirectoryEntityPath("/usr/bin")[0].(Mountable).GetMountpoint())

		// invalid dir should return nil
		assert.Nil(pt.findDirectoryEntityPath("invalid"))
	}

	{
		pt := testPartitionTables["luks+lvm"]
		assert.Equal("/", pt.findDirectoryEntityPath("/opt")[0].(Mountable).GetMountpoint())
		assert.Equal("/boot", pt.findDirectoryEntityPath("/boot")[0].(Mountable).GetMountpoint())
		assert.Equal("/boot", pt.findDirectoryEntityPath("/boot/loader")[0].(Mountable).GetMountpoint())

		ptMod := pt.Clone().(*PartitionTable)
		ptMod.Partitions = append(ptMod.Partitions, usr)
		assert.Equal("/", ptMod.findDirectoryEntityPath("/opt")[0].(Mountable).GetMountpoint())
		assert.Equal("/usr", ptMod.findDirectoryEntityPath("/usr")[0].(Mountable).GetMountpoint())
		assert.Equal("/usr", ptMod.findDirectoryEntityPath("/usr/bin")[0].(Mountable).GetMountpoint())

		// invalid dir should return nil
		assert.Nil(pt.findDirectoryEntityPath("invalid"))
	}

	{
		pt := testPartitionTables["btrfs"]
		assert.Equal("/", pt.findDirectoryEntityPath("/opt")[0].(Mountable).GetMountpoint())
		assert.Equal("/boot", pt.findDirectoryEntityPath("/boot")[0].(Mountable).GetMountpoint())
		assert.Equal("/boot", pt.findDirectoryEntityPath("/boot/loader")[0].(Mountable).GetMountpoint())

		ptMod := pt.Clone().(*PartitionTable)
		ptMod.Partitions = append(ptMod.Partitions, usr)
		assert.Equal("/", ptMod.findDirectoryEntityPath("/opt")[0].(Mountable).GetMountpoint())
		assert.Equal("/usr", ptMod.findDirectoryEntityPath("/usr")[0].(Mountable).GetMountpoint())
		assert.Equal("/usr", ptMod.findDirectoryEntityPath("/usr/bin")[0].(Mountable).GetMountpoint())

		// invalid dir should return nil
		assert.Nil(pt.findDirectoryEntityPath("invalid"))
	}

	{
		pt := PartitionTable{} // pt with no root should return nil
		assert.Nil(pt.findDirectoryEntityPath("/var"))
	}
}

func TestEnsureDirectorySizes(t *testing.T) {
	assert := assert.New(t)

	varSizes := map[string]uint64{
		"/var/lib":         uint64(3 * GiB),
		"/var/cache":       uint64(2 * GiB),
		"/var/log/journal": uint64(2 * GiB),
	}

	varAndHomeSizes := map[string]uint64{
		"/var/lib":         uint64(3 * GiB),
		"/var/cache":       uint64(2 * GiB),
		"/var/log/journal": uint64(2 * GiB),
		"/home/user/data":  uint64(10 * GiB),
	}

	{
		pt := testPartitionTables["plain"]
		pt = *pt.Clone().(*PartitionTable) // don't modify the original test data

		{
			// make sure we have the correct volume
			// guard against changes in the test pt
			rootPart := pt.Partitions[3]
			rootPayload := rootPart.Payload.(*Filesystem)

			assert.Equal("/", rootPayload.Mountpoint)
			assert.Equal(uint64(0), rootPart.Size)
		}

		{
			// add requirements for /var subdirs that are > 5 GiB
			pt.EnsureDirectorySizes(varSizes)
			rootPart := pt.Partitions[3]
			assert.Equal(uint64(7*GiB), rootPart.Size)

			// invalid
			assert.Panics(func() { pt.EnsureDirectorySizes(map[string]uint64{"invalid": uint64(300)}) })
		}
	}

	{
		pt := testPartitionTables["luks+lvm"]
		pt = *pt.Clone().(*PartitionTable) // don't modify the original test data

		{
			// make sure we have the correct volume
			// guard against changes in the test pt
			rootPart := pt.Partitions[3]
			rootLUKS := rootPart.Payload.(*LUKSContainer)
			rootVG := rootLUKS.Payload.(*LVMVolumeGroup)
			rootLV := rootVG.LogicalVolumes[0]
			rootFS := rootLV.Payload.(*Filesystem)
			homeLV := rootVG.LogicalVolumes[1]
			homeFS := homeLV.Payload.(*Filesystem)

			assert.Equal(uint64(5*GiB), rootPart.Size)
			assert.Equal("/", rootFS.Mountpoint)
			assert.Equal(uint64(2*GiB), rootLV.Size)
			assert.Equal("/home", homeFS.Mountpoint)
			assert.Equal(uint64(2*GiB), homeLV.Size)
		}

		{
			// add requirements for /var subdirs that are > 5 GiB
			pt.EnsureDirectorySizes(varAndHomeSizes)
			rootPart := pt.Partitions[3]
			rootLUKS := rootPart.Payload.(*LUKSContainer)
			rootVG := rootLUKS.Payload.(*LVMVolumeGroup)
			rootLV := rootVG.LogicalVolumes[0]
			homeLV := rootVG.LogicalVolumes[1]
			assert.Equal(uint64(17*GiB)+rootVG.MetadataSize(), rootPart.Size)
			assert.Equal(uint64(7*GiB), rootLV.Size)
			assert.Equal(uint64(10*GiB), homeLV.Size)

			// invalid
			assert.Panics(func() { pt.EnsureDirectorySizes(map[string]uint64{"invalid": uint64(300)}) })
		}
	}

	{
		pt := testPartitionTables["btrfs"]
		pt = *pt.Clone().(*PartitionTable) // don't modify the original test data

		{
			// make sure we have the correct volume
			// guard against changes in the test pt
			rootPart := pt.Partitions[3]
			rootPayload := rootPart.Payload.(*Btrfs)
			assert.Equal("/", rootPayload.Subvolumes[0].Mountpoint)
			assert.Equal(uint64(0), rootPayload.Subvolumes[0].Size)
			assert.Equal("/var", rootPayload.Subvolumes[1].Mountpoint)
			assert.Equal(uint64(5*GiB), rootPayload.Subvolumes[1].Size)
		}

		{
			// add requirements for /var subdirs that are > 5 GiB
			pt.EnsureDirectorySizes(varSizes)
			rootPart := pt.Partitions[3]
			rootPayload := rootPart.Payload.(*Btrfs)
			assert.Equal(uint64(7*GiB), rootPayload.Subvolumes[1].Size)

			// invalid
			assert.Panics(func() { pt.EnsureDirectorySizes(map[string]uint64{"invalid": uint64(300)}) })
		}
	}

}
