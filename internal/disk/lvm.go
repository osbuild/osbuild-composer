package disk

import (
	"fmt"
	"strings"
)

type LVMVolumeGroup struct {
	Name        string
	Description string

	LogicalVolumes []LVMLogicalVolume
}

func (vg *LVMVolumeGroup) IsContainer() bool {
	return true
}

func (vg *LVMVolumeGroup) Clone() Entity {
	if vg == nil {
		return nil
	}

	clone := &LVMVolumeGroup{
		Name:           vg.Name,
		Description:    vg.Description,
		LogicalVolumes: make([]LVMLogicalVolume, len(vg.LogicalVolumes)),
	}

	for idx, lv := range vg.LogicalVolumes {
		ent := lv.Clone()

		// lv.Clone() will return nil only if the logical volume is nil
		if ent == nil {
			panic(fmt.Sprintf("logical volume %d in a LVM volume group is nil; this is a programming error", idx))
		}

		lv, cloneOk := ent.(*LVMLogicalVolume)
		if !cloneOk {
			panic("LVMLogicalVolume.Clone() returned an Entity that cannot be converted to *LVMLogicalVolume; this is a programming error")
		}

		clone.LogicalVolumes[idx] = *lv
	}

	return clone
}

func (vg *LVMVolumeGroup) GetItemCount() uint {
	if vg == nil {
		return 0
	}
	return uint(len(vg.LogicalVolumes))
}

func (vg *LVMVolumeGroup) GetChild(n uint) Entity {
	if vg == nil {
		panic("LVMVolumeGroup.GetChild: nil entity")
	}
	return &vg.LogicalVolumes[n]
}

func (vg *LVMVolumeGroup) CreateMountpoint(mountpoint string, size uint64) (Entity, error) {
	if vg == nil {
		panic("LVMVolumeGroup.CreateMountpoint: nil entity")
	}
	filesystem := Filesystem{
		Type:         "xfs",
		Mountpoint:   mountpoint,
		FSTabOptions: "defaults",
		FSTabFreq:    0,
		FSTabPassNo:  0,
	}

	names := make(map[string]bool, len(vg.LogicalVolumes))
	for _, lv := range vg.LogicalVolumes {
		names[lv.Name] = true
	}

	base := lvname(mountpoint)
	var exists bool
	name := base

	// Make sure that we don't collide with an existing volume, e.g. 'home/test'
	// and /home/test_test would collide. We try 100 times and then give up. This
	// is mimicking what blivet does. See blivet/blivet.py#L1060 commit 2eb4bd4
	for i := 0; i < 100; i++ {
		exists = names[name]
		if !exists {
			break
		}

		name = fmt.Sprintf("%s%02d", base, i)
	}

	if exists {
		return nil, fmt.Errorf("could not create logical volume: name collision")
	}

	lv := LVMLogicalVolume{
		Name:    name,
		Size:    size,
		Payload: &filesystem,
	}

	vg.LogicalVolumes = append(vg.LogicalVolumes, lv)

	return &vg.LogicalVolumes[len(vg.LogicalVolumes)-1], nil
}

func (vg *LVMVolumeGroup) MetadataSize() uint64 {
	if vg == nil {
		return 0
	}

	// LVM2 allows for a lot of customizations that will affect the size
	// of the metadata and its location and thus the start of the physical
	// extent. For now we assume the default which results in a start of
	// the physical extent 1 MiB
	return 1024 * 1024
}

type LVMLogicalVolume struct {
	Name    string
	Size    uint64
	Payload Entity
}

func (lv *LVMLogicalVolume) IsContainer() bool {
	return true
}

func (lv *LVMLogicalVolume) Clone() Entity {
	if lv == nil {
		return nil
	}
	return &LVMLogicalVolume{
		Name:    lv.Name,
		Size:    lv.Size,
		Payload: lv.Payload.Clone(),
	}
}

func (lv *LVMLogicalVolume) GetItemCount() uint {
	if lv == nil || lv.Payload == nil {
		return 0
	}
	return 1
}

func (lv *LVMLogicalVolume) GetChild(n uint) Entity {
	if n != 0 || lv == nil {
		panic(fmt.Sprintf("invalid child index for LVMLogicalVolume: %d != 0", n))
	}
	return lv.Payload
}

func (lv *LVMLogicalVolume) GetSize() uint64 {
	if lv == nil {
		return 0
	}
	return lv.Size
}

func (lv *LVMLogicalVolume) EnsureSize(s uint64) bool {
	if lv == nil {
		panic("LVMLogicalVolume.EnsureSize: nil entity")
	}
	if s > lv.Size {
		lv.Size = s
		return true
	}
	return false
}

// lvname returns a name for a logical volume based on the mountpoint.
func lvname(path string) string {
	if path == "/" {
		return "rootlv"
	}

	path = strings.TrimLeft(path, "/")
	return strings.ReplaceAll(path, "/", "_") + "lv"
}
