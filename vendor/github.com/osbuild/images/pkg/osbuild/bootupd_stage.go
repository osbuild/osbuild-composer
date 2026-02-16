package osbuild

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/platform"
)

type BootupdStageOptionsBios struct {
	Device    string `json:"device"`
	Partition int    `json:"partition,omitempty"`
}

type BootupdStageOptions struct {
	Deployment    *OSTreeDeployment        `json:"deployment,omitempty"`
	StaticConfigs bool                     `json:"static-configs"`
	Bios          *BootupdStageOptionsBios `json:"bios,omitempty"`
}

func (BootupdStageOptions) isStageOptions() {}

func (opts *BootupdStageOptions) validate(devices map[string]Device) error {
	if opts.Bios != nil && opts.Bios.Device != "" {
		if _, ok := devices[opts.Bios.Device]; !ok {
			var devnames []string
			for devname := range devices {
				devnames = append(devnames, devname)
			}
			slices.Sort(devnames)
			return fmt.Errorf("cannot find expected device %q for bootupd bios option in %v", opts.Bios.Device, devnames)
		}
	}
	return nil
}

// validateBootupdMounts ensures that all required mounts for the bootup
// stage are generated. Right now the stage requires root, boot and boot/efi
// to find all the bootloader configs
func validateBootupdMounts(mounts []Mount, pf platform.Platform) error {
	requiredMounts := map[string]bool{
		"/": true,
	}
	if pf.GetUEFIVendor() != "" {
		requiredMounts["/boot/efi"] = true
	}
	for _, mnt := range mounts {
		delete(requiredMounts, mnt.Target)
	}
	if len(requiredMounts) != 0 {
		var missingMounts []string
		for mnt := range requiredMounts {
			missingMounts = append(missingMounts, mnt)
		}
		slices.Sort(missingMounts)
		return fmt.Errorf("required mounts for bootupd stage %v missing", missingMounts)
	}
	return nil
}

// NewBootupdStage creates a new stage for the org.osbuild.bootupd stage. It
// requires a mount setup of "/", "/boot" and "/boot/efi" right now so that
// bootupd can find and install all required bootloader bits.
func NewBootupdStage(opts *BootupdStageOptions, devices map[string]Device, mounts []Mount, pltf platform.Platform) (*Stage, error) {
	if err := validateBootupdMounts(mounts, pltf); err != nil {
		return nil, err
	}
	if err := opts.validate(devices); err != nil {
		return nil, err
	}

	return &Stage{
		Type:    "org.osbuild.bootupd",
		Options: opts,
		Devices: devices,
		Mounts:  mounts,
	}, nil
}

func genMountsForBootupd(source string, pt *disk.PartitionTable) ([]Mount, error) {
	mounts := make([]Mount, 0, len(pt.Partitions))
	// note that we are not using pt.forEachMountable() here because we
	// need to keep track of the partition number (even if it's not
	// mountable)
	for idx, part := range pt.Partitions {
		if part.Payload == nil {
			continue
		}

		// TODO: support things like LUKS here via supporting "disk.Container"?
		switch payload := part.Payload.(type) {
		case disk.Mountable:
			mount, err := genOsbuildMount(source, payload)
			if err != nil {
				return nil, err
			}
			mount.Partition = common.ToPtr(idx + 1)
			mounts = append(mounts, *mount)
		case *disk.Btrfs:
			for i := range payload.Subvolumes {
				mount, err := genOsbuildMount(source, &payload.Subvolumes[i])
				if err != nil {
					return nil, err
				}
				mount.Partition = common.ToPtr(idx + 1)
				mounts = append(mounts, *mount)
			}
		case *disk.LVMVolumeGroup:
			for i := range payload.LogicalVolumes {
				lv := &payload.LogicalVolumes[i]
				switch payload := lv.Payload.(type) {
				case disk.Mountable:
					mount, err := genOsbuildMount(lv.Name, payload)
					if err != nil {
						return nil, err
					}
					mount.Source = lv.Name
					mounts = append(mounts, *mount)
				case *disk.Swap, *disk.Raw:
					// nothing to do
				default:
					return nil, fmt.Errorf("expected LV payload %+[1]v to be mountable or swap, got %[1]T", lv.Payload)
				}
			}
		case *disk.Swap, *disk.Raw:
			// nothing to do
		default:
			return nil, fmt.Errorf("type %T not supported by bootupd handling yet", part.Payload)
		}
	}
	// this must be sorted in so that mounts do not shadow each other
	slices.SortFunc(mounts, func(a, b Mount) int {
		return cmp.Compare(a.Target, b.Target)
	})

	return mounts, nil
}

func genDevicesForBootupd(filename, devName string, pt *disk.PartitionTable) (map[string]Device, error) {
	devices := map[string]Device{
		devName: Device{
			Type: "org.osbuild.loopback",
			Options: &LoopbackDeviceOptions{
				Filename: filename,
				Partscan: true,
			},
		},
	}
	for idx, part := range pt.Partitions {
		switch payload := part.Payload.(type) {
		case *disk.LVMVolumeGroup:
			for _, lv := range payload.LogicalVolumes {
				// partitions start with "1", so add "1"
				partNum := idx + 1
				devices[lv.Name] = *NewLVM2LVDevice(devName, &LVM2LVDeviceOptions{Volume: lv.Name, VGPartnum: common.ToPtr(partNum)})
			}
		default:
			// nothing
		}
	}

	return devices, nil
}

func GenBootupdDevicesMounts(filename string, pt *disk.PartitionTable, pltf platform.Platform) (map[string]Device, []Mount, error) {
	devName := "disk"
	devices, err := genDevicesForBootupd(filename, devName, pt)
	if err != nil {
		return nil, nil, err
	}
	mounts, err := genMountsForBootupd(devName, pt)
	if err != nil {
		return nil, nil, err
	}
	if err := validateBootupdMounts(mounts, pltf); err != nil {
		return nil, nil, err
	}

	return devices, mounts, nil
}
