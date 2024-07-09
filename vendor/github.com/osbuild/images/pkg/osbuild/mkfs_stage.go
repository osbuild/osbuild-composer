package osbuild

import (
	"fmt"
	"strings"

	"github.com/osbuild/images/pkg/disk"
)

// GenMkfsStages generates a list of org.mkfs.* stages based on a
// partition table description for a single device node
// filename is the path to the underlying image file (to be used as a source for the loopback device)
func GenMkfsStages(pt *disk.PartitionTable, filename string) []*Stage {
	stages := make([]*Stage, 0, len(pt.Partitions))

	processedBtrfsPartitions := make(map[string]bool)
	genStage := func(mnt disk.Mountable, path []disk.Entity) error {
		t := mnt.GetFSType()
		var stage *Stage

		stageDevices, lastName := getDevices(path, filename, true)

		// The last device in the chain must be named "device", because that's the device that mkfs stages run on.
		// See their schema for reference.
		lastDevice := stageDevices[lastName]
		delete(stageDevices, lastName)
		stageDevices["device"] = lastDevice

		fsSpec := mnt.GetFSSpec()
		switch t {
		case "xfs":
			options := &MkfsXfsStageOptions{
				UUID:  fsSpec.UUID,
				Label: fsSpec.Label,
			}
			stage = NewMkfsXfsStage(options, stageDevices)
		case "vfat":
			options := &MkfsFATStageOptions{
				VolID: strings.Replace(fsSpec.UUID, "-", "", -1),
			}
			stage = NewMkfsFATStage(options, stageDevices)
		case "btrfs":
			// the disk library allows only subvolumes as Mountable, so we need to find the underlying btrfs partition
			// and mkfs it
			btrfsPart := findBtrfsPartition(path)
			if btrfsPart == nil {
				panic(fmt.Sprintf("found btrfs subvolume without btrfs partition: %s", mnt.GetMountpoint()))
			}

			// btrfs partitions can be shared between multiple subvolumes, so we need to make sure we only create
			// one
			if processedBtrfsPartitions[btrfsPart.UUID] {
				return nil
			}
			processedBtrfsPartitions[btrfsPart.UUID] = true

			options := &MkfsBtrfsStageOptions{
				UUID:  btrfsPart.UUID,
				Label: btrfsPart.Label,
			}
			stage = NewMkfsBtrfsStage(options, stageDevices)
		case "ext4":
			options := &MkfsExt4StageOptions{
				UUID:  fsSpec.UUID,
				Label: fsSpec.Label,
			}
			stage = NewMkfsExt4Stage(options, stageDevices)
		default:
			panic("unknown fs type " + t)
		}
		stages = append(stages, stage)

		return nil
	}

	_ = pt.ForEachMountable(genStage) // genStage always returns nil
	return stages
}

func findBtrfsPartition(path []disk.Entity) *disk.Btrfs {
	for _, e := range path {
		if btrfsPartition, ok := e.(*disk.Btrfs); ok {
			return btrfsPartition
		}
	}
	return nil
}
