package disk

import (
	"fmt"
	"math/rand"
	"strings"
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
	"small": {
		{
			Mountpoint: "/opt",
			MinSize:    20 * MiB,
		},
		{
			Mountpoint: "/home",
			MinSize:    500 * MiB,
		},
	},
	"empty": nil,
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

	sizeCheckCB := func(mnt Mountable, path []Entity) error {
		if strings.HasPrefix(mnt.GetMountpoint(), "/boot") {
			// /boot and subdirectories is exempt from this rule
			return nil
		}
		// go up the path and check every sizeable
		for idx, ent := range path {
			if sz, ok := ent.(Sizeable); ok {
				size := sz.GetSize()
				if size < 1*GiB {
					return fmt.Errorf("entity %d in the path from %s is smaller than the minimum 1 GiB (%d)", idx, mnt.GetMountpoint(), size)
				}
			}
		}
		return nil
	}

	sumSizes := func(bp []blueprint.FilesystemCustomization) (sum uint64) {
		for _, mnt := range bp {
			sum += mnt.MinSize
		}
		return sum
	}
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(13))
	for ptName := range testPartitionTables {
		pt := testPartitionTables[ptName]
		for bpName, bp := range testBlueprints {
			mpt, err := NewPartitionTable(&pt, bp, uint64(13*MiB), false, rng)
			assert.NoError(err, "Partition table generation failed: PT %q BP %q (%s)", ptName, bpName, err)
			assert.NotNil(mpt, "Partition table generation failed: PT %q BP %q (nil partition table)", ptName, bpName)
			assert.Greater(mpt.GetSize(), sumSizes(bp))

			assert.NotNil(mpt.Type, "Partition table generation failed: PT %q BP %q (nil partition table type)", ptName, bpName)

			mnt := pt.FindMountable("/")
			assert.NotNil(mnt, "PT %q BP %q: failed to find root mountable", ptName, bpName)

			assert.NoError(mpt.ForEachMountable(sizeCheckCB))
		}
	}
}

func TestCreatePartitionTableLVMify(t *testing.T) {
	assert := assert.New(t)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(13))
	for bpName, tbp := range testBlueprints {
		for ptName := range testPartitionTables {
			pt := testPartitionTables[ptName]

			if tbp != nil && (ptName == "btrfs" || ptName == "luks") {
				assert.Panics(func() {
					_, _ = NewPartitionTable(&pt, tbp, uint64(13*MiB), true, rng)
				}, fmt.Sprintf("PT %q BP %q: should panic", ptName, bpName))
				continue
			}

			mpt, err := NewPartitionTable(&pt, tbp, uint64(13*MiB), true, rng)
			assert.NoError(err, "PT %q BP %q: Partition table generation failed: (%s)", ptName, bpName, err)

			rootPath := entityPath(mpt, "/")
			if rootPath == nil {
				panic(fmt.Sprintf("PT %q BP %q: no root mountpoint", ptName, bpName))
			}

			bootPath := entityPath(mpt, "/boot")
			if tbp != nil && bootPath == nil {
				panic(fmt.Sprintf("PT %q BP %q: no boot mountpoint", ptName, bpName))
			}

			if tbp != nil {
				parent := rootPath[1]
				_, ok := parent.(*LVMLogicalVolume)
				assert.True(ok, "PT %q BP %q: root's parent (%q) is not an LVM logical volume", ptName, bpName, parent)
			}
		}
	}
}

func TestMinimumSizes(t *testing.T) {
	assert := assert.New(t)

	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(13))
	pt := testPartitionTables["plain"]

	type testCase struct {
		Blueprint        []blueprint.FilesystemCustomization
		ExpectedMinSizes map[string]uint64
	}

	testCases := []testCase{
		{ // specify small /usr -> / and /usr get default size
			Blueprint: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/usr",
					MinSize:    1 * MiB,
				},
			},
			ExpectedMinSizes: map[string]uint64{
				"/usr": 2 * GiB,
				"/":    1 * GiB,
			},
		},
		{ // specify small / and /usr -> / and /usr get default size
			Blueprint: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    1 * MiB,
				},
				{
					Mountpoint: "/usr",
					MinSize:    1 * KiB,
				},
			},
			ExpectedMinSizes: map[string]uint64{
				"/usr": 2 * GiB,
				"/":    1 * GiB,
			},
		},
		{ // big /usr -> / gets default size
			Blueprint: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/usr",
					MinSize:    10 * GiB,
				},
			},
			ExpectedMinSizes: map[string]uint64{
				"/usr": 10 * GiB,
				"/":    1 * GiB,
			},
		},
		{
			Blueprint: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/",
					MinSize:    10 * GiB,
				},
				{
					Mountpoint: "/home",
					MinSize:    1 * MiB,
				},
			},
			ExpectedMinSizes: map[string]uint64{
				"/":     10 * GiB,
				"/home": 1 * GiB,
			},
		},
		{ // no separate /usr and no size for / -> / gets sum of default sizes for / and /usr
			Blueprint: []blueprint.FilesystemCustomization{
				{
					Mountpoint: "/opt",
					MinSize:    10 * GiB,
				},
			},
			ExpectedMinSizes: map[string]uint64{
				"/opt": 10 * GiB,
				"/":    3 * GiB,
			},
		},
	}

	for idx, tc := range testCases {
		{ // without LVM
			mpt, err := NewPartitionTable(&pt, tc.Blueprint, uint64(3*GiB), false, rng)
			assert.NoError(err)
			for mnt, minSize := range tc.ExpectedMinSizes {
				path := entityPath(mpt, mnt)
				assert.NotNil(path, "[%d] mountpoint %q not found", idx, mnt)
				parent := path[1]
				part, ok := parent.(*Partition)
				assert.True(ok, "%q parent (%v) is not a partition", mnt, parent)
				assert.GreaterOrEqual(part.GetSize(), minSize,
					"[%d] %q size %d should be greater or equal to %d", idx, mnt, part.GetSize(), minSize)
			}
		}

		{ // with LVM
			mpt, err := NewPartitionTable(&pt, tc.Blueprint, uint64(3*GiB), true, rng)
			assert.NoError(err)
			for mnt, minSize := range tc.ExpectedMinSizes {
				path := entityPath(mpt, mnt)
				assert.NotNil(path, "[%d] mountpoint %q not found", idx, mnt)
				parent := path[1]
				part, ok := parent.(*LVMLogicalVolume)
				assert.True(ok, "[%d] %q parent (%v) is not an LVM logical volume", idx, mnt, parent)
				assert.GreaterOrEqual(part.GetSize(), minSize,
					"[%d] %q size %d should be greater or equal to %d", idx, mnt, part.GetSize(), minSize)
			}
		}
	}
}

func TestNewBootWithSizeLVMify(t *testing.T) {
	pt := testPartitionTables["plain-noboot"]
	assert := assert.New(t)

	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(rand.NewSource(13))

	custom := []blueprint.FilesystemCustomization{
		{
			Mountpoint: "/boot",
			MinSize:    700 * MiB,
		},
	}

	mpt, err := NewPartitionTable(&pt, custom, uint64(3*GiB), true, rng)
	assert.NoError(err)

	for idx, c := range custom {
		mnt, minSize := c.Mountpoint, c.MinSize
		path := entityPath(mpt, mnt)
		assert.NotNil(path, "[%d] mountpoint %q not found", idx, mnt)
		parent := path[1]
		part, ok := parent.(*Partition)
		assert.True(ok, "%q parent (%v) is not a partition", mnt, parent)
		assert.GreaterOrEqual(part.GetSize(), minSize,
			"[%d] %q size %d should be greater or equal to %d", idx, mnt, part.GetSize(), minSize)
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
