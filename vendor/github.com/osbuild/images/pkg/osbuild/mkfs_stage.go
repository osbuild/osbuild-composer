package osbuild

import (
	"fmt"
	"strings"

	"github.com/osbuild/images/pkg/disk"
)

// GenFsStages generates a list of stages that create the filesystem and other
// related entities. Specifically, it creates stages for:
//   - org.osbuild.mkfs.*: for all filesystems and btrfs volumes
//   - org.osbuild.btrfs.subvol: for all btrfs subvolumes
//   - org.osbuild.mkswap: for swap areas
func GenFsStages(pt *disk.PartitionTable, filename string) []*Stage {
	stages := make([]*Stage, 0, len(pt.Partitions))

	genStage := func(ent disk.Entity, path []disk.Entity) error {
		switch e := ent.(type) {
		case *disk.Filesystem:
			// TODO: extract last device renaming into helper
			stageDevices, lastName := getDevices(path, filename, true)

			// The last device in the chain must be named "device", because that's
			// the device that mkfs stages run on. See the stage schemas for
			// reference.
			lastDevice := stageDevices[lastName]
			delete(stageDevices, lastName)
			stageDevices["device"] = lastDevice

			switch e.GetFSType() {
			case "xfs":
				options := &MkfsXfsStageOptions{
					UUID:  e.UUID,
					Label: e.Label,
				}
				stages = append(stages, NewMkfsXfsStage(options, stageDevices))
			case "vfat":
				options := &MkfsFATStageOptions{
					VolID: strings.Replace(e.UUID, "-", "", -1),
				}
				stages = append(stages, NewMkfsFATStage(options, stageDevices))
			case "ext4":
				options := &MkfsExt4StageOptions{
					UUID:  e.UUID,
					Label: e.Label,
				}
				stages = append(stages, NewMkfsExt4Stage(options, stageDevices))
			default:
				panic(fmt.Sprintf("unknown fs type: %s", e.GetFSType()))
			}
		case *disk.Btrfs:
			stageDevices, lastName := getDevices(path, filename, true)

			// The last device in the chain must be named "device", because that's
			// the device that mkfs stages run on. See the stage schemas for
			// reference.
			lastDevice := stageDevices[lastName]
			delete(stageDevices, lastName)
			stageDevices["device"] = lastDevice

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
			// TODO: extract last device renaming into helper
			stageDevices, lastName := getDevices(path, filename, true)

			// The last device in the chain must be named "device", because that's
			// the device that the mkswap stage runs on. See the stage schema
			// for reference.
			lastDevice := stageDevices[lastName]
			delete(stageDevices, lastName)
			stageDevices["device"] = lastDevice

			options := &MkswapStageOptions{
				UUID:  e.UUID,
				Label: e.Label,
			}
			stages = append(stages, NewMkswapStage(options, stageDevices))
		}
		return nil
	}

	_ = pt.ForEachEntity(genStage) // genStage always returns nil
	return stages

}
