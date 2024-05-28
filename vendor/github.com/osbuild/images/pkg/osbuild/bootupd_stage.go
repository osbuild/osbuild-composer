package osbuild

import (
	"fmt"
	"sort"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
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
			sort.Strings(devnames)
			return fmt.Errorf("cannot find expected device %q for bootupd bios option in %v", opts.Bios.Device, devnames)
		}
	}
	return nil
}

// validateBootupdMounts ensures that all required mounts for the bootup
// stage are generated. Right now the stage requires root, boot and boot/efi
// to find all the bootloader configs
func validateBootupdMounts(mounts []Mount) error {
	requiredMounts := map[string]bool{
		"/":         true,
		"/boot":     true,
		"/boot/efi": true,
	}
	for _, mnt := range mounts {
		delete(requiredMounts, mnt.Target)
	}
	if len(requiredMounts) != 0 {
		var missingMounts []string
		for mnt := range requiredMounts {
			missingMounts = append(missingMounts, mnt)
		}
		sort.Strings(missingMounts)
		return fmt.Errorf("required mounts for bootupd stage %v missing", missingMounts)
	}
	return nil
}

// NewBootupdStage creates a new stage for the org.osbuild.bootupd stage. It
// requires a mount setup of "/", "/boot" and "/boot/efi" right now so that
// bootupd can find and install all required bootloader bits.
func NewBootupdStage(opts *BootupdStageOptions, devices map[string]Device, mounts []Mount) (*Stage, error) {
	if err := validateBootupdMounts(mounts); err != nil {
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

		// TODO: support things like LVM here via supporting "disk.Container"
		switch payload := part.Payload.(type) {
		case disk.Mountable:
			mount, err := genOsbuildMount(source, payload)
			if err != nil {
				return nil, err
			}
			mount.Partition = common.ToPtr(idx + 1)
			mounts = append(mounts, *mount)
		default:
			return nil, fmt.Errorf("type %T not supported by bootupd handling yet", part.Payload)
		}
	}
	// this must be sorted in so that mounts do not shadow each other
	sort.Slice(mounts, func(i, j int) bool {
		return mounts[i].Target < mounts[j].Target
	})

	return mounts, nil
}

func GenBootupdDevicesMounts(filename string, pt *disk.PartitionTable) (map[string]Device, []Mount, error) {
	devName := "disk"
	devices := map[string]Device{
		devName: Device{
			Type: "org.osbuild.loopback",
			Options: &LoopbackDeviceOptions{
				Filename: filename,
				Partscan: true,
			},
		},
	}
	mounts, err := genMountsForBootupd(devName, pt)
	if err != nil {
		return nil, nil, err
	}
	if err := validateBootupdMounts(mounts); err != nil {
		return nil, nil, err
	}

	return devices, mounts, nil
}
