package disk

import "fmt"

type LVMVolumeGroup struct {
	Name        string
	Description string

	LogicalVolumes []LVMLogicalVolume
}

func (vg *LVMVolumeGroup) IsContainer() bool {
	return true
}

func (vg *LVMVolumeGroup) GetItemCount() uint {
	return uint(len(vg.LogicalVolumes))
}

func (vg *LVMVolumeGroup) GetChild(n uint) Entity {
	return &vg.LogicalVolumes[n]
}

func (vg *LVMVolumeGroup) CreateVolume(mountpoint string, size uint64) (Entity, error) {
	filesystem := Filesystem{
		Type:         "xfs",
		Mountpoint:   mountpoint,
		FSTabOptions: "defaults",
		FSTabFreq:    0,
		FSTabPassNo:  0,
	}

	lv := LVMLogicalVolume{
		Size:    size,
		Payload: &filesystem,
	}

	vg.LogicalVolumes = append(vg.LogicalVolumes, lv)

	return &vg.LogicalVolumes[len(vg.LogicalVolumes)-1], nil
}

type LVMLogicalVolume struct {
	Name    string
	Size    uint64
	Payload Entity
}

func (lv *LVMLogicalVolume) IsContainer() bool {
	return true
}

func (lv *LVMLogicalVolume) GetItemCount() uint {
	if lv.Payload == nil {
		return 0
	}
	return 1
}

func (lv *LVMLogicalVolume) GetChild(n uint) Entity {
	if n != 0 {
		panic(fmt.Sprintf("invalid child index for LVMLogicalVolume: %d != 0", n))
	}
	return lv.Payload
}

func (lv *LVMLogicalVolume) GetSize() uint64 {
	return lv.Size
}

func (lv *LVMLogicalVolume) EnsureSize(s uint64) bool {
	if s > lv.Size {
		lv.Size = s
		return true
	}
	return false
}
