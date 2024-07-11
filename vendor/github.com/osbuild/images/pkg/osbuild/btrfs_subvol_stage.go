package osbuild

import (
	"github.com/osbuild/images/pkg/disk"
)

type BtrfsSubVolOptions struct {
	Subvolumes []BtrfsSubVol `json:"subvolumes"`
}

type BtrfsSubVol struct {
	Name string `json:"name"`
}

func (BtrfsSubVolOptions) isStageOptions() {}

func NewBtrfsSubVol(options *BtrfsSubVolOptions, devices *map[string]Device, mounts *[]Mount) *Stage {
	return &Stage{
		Type:    "org.osbuild.btrfs.subvol",
		Options: options,
		Devices: *devices,
		Mounts:  *mounts,
	}
}

func GenBtrfsSubVolStage(filename string, pt *disk.PartitionTable) *Stage {
	var subvolumes []BtrfsSubVol

	genStage := func(mnt disk.Mountable, path []disk.Entity) error {
		if mnt.GetFSType() != "btrfs" {
			return nil
		}

		btrfs := mnt.(*disk.BtrfsSubvolume)
		subvolumes = append(subvolumes, BtrfsSubVol{Name: "/" + btrfs.Name})

		return nil
	}

	_ = pt.ForEachMountable(genStage)

	if len(subvolumes) == 0 {
		return nil
	}

	devices, mounts := genBtrfsMountDevices(filename, pt)

	return NewBtrfsSubVol(&BtrfsSubVolOptions{subvolumes}, devices, mounts)
}

func genBtrfsMountDevices(filename string, pt *disk.PartitionTable) (*map[string]Device, *[]Mount) {
	devices := make(map[string]Device, len(pt.Partitions))
	mounts := make([]Mount, 0, len(pt.Partitions))
	genMounts := func(ent disk.Entity, path []disk.Entity) error {
		if _, isBtrfs := ent.(*disk.Btrfs); !isBtrfs {
			return nil
		}

		stageDevices, name := getDevices(path, filename, false)

		mounts = append(mounts, *NewBtrfsMount(name, name, "/", "", ""))

		// update devices map with new elements from stageDevices
		for devName := range stageDevices {
			devices[devName] = stageDevices[devName]
		}
		return nil
	}

	_ = pt.ForEachEntity(genMounts)

	return &devices, &mounts
}
