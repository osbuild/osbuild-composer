package osbuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
)

// Helper to create the `devices` option for the stage with the right
// name such that the last device is the target.
func getDevicesForFsStage(path []disk.Entity, filename string) map[string]Device {
	stageDevices, lastName := getDevices(path, filename, true)

	// The last device in the chain must be named "device",
	// because that's the device that mkfs and write-device stages
	// run on. See the stage schemas for reference.
	lastDevice := stageDevices[lastName]
	delete(stageDevices, lastName)
	stageDevices["device"] = lastDevice

	return stageDevices
}

// GenFsStages generates a list of stages that create the filesystem and other
// related entities. Specifically, it creates stages for:
//   - org.osbuild.mkfs.*: for all filesystems and btrfs volumes
//   - org.osbuild.btrfs.subvol: for all btrfs subvolumes
//   - org.osbuild.mkswap: for swap areas
func GenFsStages(pt *disk.PartitionTable, filename string, soucePipeline string) []*Stage {
	stages := make([]*Stage, 0, len(pt.Partitions))

	genStage := func(ent disk.Entity, path []disk.Entity) error {
		switch e := ent.(type) {
		case *disk.Filesystem:
			stageDevices := getDevicesForFsStage(path, filename)

			// Make a copy so we can mark the ones we handled
			mkfsOptions := e.MkfsOptions
			switch e.GetFSType() {
			case "xfs":
				options := &MkfsXfsStageOptions{
					UUID:  e.UUID,
					Label: e.Label,
				}
				stages = append(stages, NewMkfsXfsStage(options, stageDevices))
			case "vfat":
				options := &MkfsFATStageOptions{
					VolID: strings.ReplaceAll(e.UUID, "-", ""),
					Label: e.Label,
				}
				if mkfsOptions.Geometry != nil {
					options.Geometry = &MkfsFATStageGeometryOptions{
						Heads:           e.MkfsOptions.Geometry.Heads,
						SectorsPerTrack: e.MkfsOptions.Geometry.SectorsPerTrack,
					}
					mkfsOptions.Geometry = nil // Handled
				}

				stages = append(stages, NewMkfsFATStage(options, stageDevices))
			case "ext4":
				options := &MkfsExt4StageOptions{
					UUID:  e.UUID,
					Label: e.Label,
				}
				if mkfsOptions.Verity {
					options.Verity = common.ToPtr(true)
					mkfsOptions.Verity = false // Handled
				}

				stages = append(stages, NewMkfsExt4Stage(options, stageDevices))
			default:
				panic(fmt.Sprintf("unknown fs type: %s for %s", e.GetFSType(), e.GetMountpoint()))
			}

			if mkfsOptions.Geometry != nil {
				panic(fmt.Sprintf("fs type: %s does not support geometry option", e.GetFSType()))
			}
			if mkfsOptions.Verity {
				panic(fmt.Sprintf("fs type: %s does not support verity option", e.GetFSType()))
			}

		case *disk.Btrfs:
			stageDevices := getDevicesForFsStage(path, filename)

			options := &MkfsBtrfsStageOptions{
				UUID:  e.UUID,
				Label: e.Label,
			}
			stages = append(stages, NewMkfsBtrfsStage(options, stageDevices))
			// Handle subvolumes here directly instead of collecting them in
			// their own case, since we already have access to the parent volume.
			subvolumes := make([]BtrfsSubVol, len(e.Subvolumes))
			for idx, subvol := range e.Subvolumes {
				subvolumes[idx] = BtrfsSubVol{Name: "/" + strings.TrimLeft(subvol.Name, "/")}
			}

			// Subvolume creation does not require locking the device, nor does
			// it require the renaming to "device", but let's reuse the volume
			// device for convenience
			mount := *NewBtrfsMount("volume", "device", "/", "", "")
			stages = append(stages, NewBtrfsSubVol(&BtrfsSubVolOptions{subvolumes}, &stageDevices, &[]Mount{mount}))
		case *disk.Swap:
			stageDevices := getDevicesForFsStage(path, filename)

			options := &MkswapStageOptions{
				UUID:  e.UUID,
				Label: e.Label,
			}
			stages = append(stages, NewMkswapStage(options, stageDevices))
		case *disk.Raw:
			stageDevices := getDevicesForFsStage(path, filename)

			inputName := "tree"
			options := &WriteDeviceStageOptions{
				From: fmt.Sprintf("input://%s", filepath.Join(inputName, e.SourcePath)),
			}
			inputs := NewPipelineTreeInputs(inputName, soucePipeline)
			stages = append(stages, NewWriteDeviceStage(options, inputs, stageDevices))
		}
		return nil
	}

	_ = pt.ForEachEntity(genStage) // genStage always returns nil
	return stages

}
