package osbuild2

import (
	"strings"

	"github.com/osbuild/osbuild-composer/internal/disk"
)

// GenMkfsStages generates a list of org.mkfs.* stages based on a
// partition table description for a single device node
func GenMkfsStages(pt *disk.PartitionTable, device *Device) []*Stage {
	stages := make([]*Stage, 0, len(pt.Partitions))

	// assume loopback device for simplicity since it's the only one currently supported
	// panic if the conversion fails
	devOptions, ok := device.Options.(*LoopbackDeviceOptions)
	if !ok {
		panic("GenMkfsStages: failed to convert device options to loopback options")
	}

	for _, p := range pt.Partitions {
		if p.Payload == nil {
			// no filesystem for partition (e.g., BIOS boot)
			continue
		}
		var stage *Stage
		stageDevice := NewLoopbackDevice(
			&LoopbackDeviceOptions{
				Filename: devOptions.Filename,
				Start:    pt.BytesToSectors(p.Start),
				Size:     pt.BytesToSectors(p.Size),
			},
		)
		switch p.Payload.Type {
		case "xfs":
			options := &MkfsXfsStageOptions{
				UUID:  p.Payload.UUID,
				Label: p.Payload.Label,
			}
			stage = NewMkfsXfsStage(options, stageDevice)
		case "vfat":
			options := &MkfsFATStageOptions{
				VolID: strings.Replace(p.Payload.UUID, "-", "", -1),
			}
			stage = NewMkfsFATStage(options, stageDevice)
		case "btrfs":
			options := &MkfsBtrfsStageOptions{
				UUID:  p.Payload.UUID,
				Label: p.Payload.Label,
			}
			stage = NewMkfsBtrfsStage(options, stageDevice)
		case "ext4":
			options := &MkfsExt4StageOptions{
				UUID:  p.Payload.UUID,
				Label: p.Payload.Label,
			}
			stage = NewMkfsExt4Stage(options, stageDevice)
		default:
			panic("unknown fs type " + p.Type)
		}
		stages = append(stages, stage)
	}
	return stages
}
