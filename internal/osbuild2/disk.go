package osbuild2

import (
	"fmt"

	"github.com/osbuild/osbuild-composer/internal/disk"
)

// sfdiskStageOptions creates the options and devices properties for an
// org.osbuild.sfdisk stage based on a partition table description
func sfdiskStageOptions(pt *disk.PartitionTable) *SfdiskStageOptions {
	partitions := make([]Partition, len(pt.Partitions))
	for idx, p := range pt.Partitions {
		partitions[idx] = Partition{
			Bootable: p.Bootable,
			Start:    pt.BytesToSectors(p.Start),
			Size:     pt.BytesToSectors(p.Size),
			Type:     p.Type,
			UUID:     p.UUID,
		}
	}
	stageOptions := &SfdiskStageOptions{
		Label:      pt.Type,
		UUID:       pt.UUID,
		Partitions: partitions,
	}

	return stageOptions
}

func GenImagePrepareStages(pt *disk.PartitionTable, filename string) []*Stage {
	stages := make([]*Stage, 0)

	// create an empty file of the given size via `org.osbuild.truncate`
	stage := NewTruncateStage(
		&TruncateStageOptions{
			Filename: filename,
			Size:     fmt.Sprintf("%d", pt.Size),
		})

	stages = append(stages, stage)

	// create the partition layout in the empty file
	sfOptions := sfdiskStageOptions(pt)
	loopback := NewLoopbackDevice(
		&LoopbackDeviceOptions{Filename: filename},
	)

	sfdisk := NewSfdiskStage(sfOptions, loopback)
	stages = append(stages, sfdisk)

	// Generate all the needed "devices", like LUKS2 and LVM2
	s := GenDeviceCreationStages(pt, filename)
	stages = append(stages, s...)

	// Generate all the filesystems on partitons and devices
	s = GenMkfsStages(pt, loopback)
	stages = append(stages, s...)

	return stages
}

func GenImageFinishStages(pt *disk.PartitionTable, filename string) []*Stage {
	return GenDeviceFinishStages(pt, filename)
}
