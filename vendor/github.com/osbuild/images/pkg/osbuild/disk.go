package osbuild

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
)

type MountConfiguration uint

const ( // MountConfiguration enum
	// fstab is default/0 to keep compatibility
	MOUNT_CONFIGURATION_FSTAB MountConfiguration = iota
	MOUNT_CONFIGURATION_UNITS
	MOUNT_CONFIGURATION_NONE
)

func (c MountConfiguration) String() string {
	switch c {
	case MOUNT_CONFIGURATION_FSTAB:
		return "fstab"
	case MOUNT_CONFIGURATION_UNITS:
		return "units"
	case MOUNT_CONFIGURATION_NONE:
		return "none"
	default:
		panic(fmt.Sprintf("unknown or unsupported mount configuration enum value %d", c))
	}
}

func (c *MountConfiguration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	new, err := NewMountConfiguration(s)
	if err != nil {
		return err
	}
	*c = new
	return nil
}

func (c *MountConfiguration) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(c, unmarshal)
}

func NewMountConfiguration(s string) (MountConfiguration, error) {
	switch s {
	case "fstab":
		return MOUNT_CONFIGURATION_FSTAB, nil
	case "units":
		return MOUNT_CONFIGURATION_UNITS, nil
	case "none":
		return MOUNT_CONFIGURATION_NONE, nil
	default:
		return 0, fmt.Errorf("unknown or unsupported filesystem type name: %s", s)
	}
}

// sfdiskStageOptions creates the options and devices properties for an
// org.osbuild.sfdisk stage based on a partition table description
func sfdiskStageOptions(pt *disk.PartitionTable) *SfdiskStageOptions {
	partitions := make([]SfdiskPartition, len(pt.Partitions))
	for idx, p := range pt.Partitions {
		partitions[idx] = SfdiskPartition{
			Bootable: p.Bootable,
			Start:    pt.BytesToSectors(p.Start),
			Size:     pt.BytesToSectors(p.Size),
			Type:     p.Type,
			UUID:     p.UUID,
			Name:     p.Label,
			Attrs:    p.Attrs,
		}
	}
	stageOptions := &SfdiskStageOptions{
		Label:      pt.Type.String(),
		UUID:       pt.UUID,
		Partitions: partitions,
	}

	return stageOptions
}

// sgdiskStageOptions creates the options and devices properties for an
// org.osbuild.sgdisk stage based on a partition table description
func sgdiskStageOptions(pt *disk.PartitionTable) *SgdiskStageOptions {
	partitions := make([]SgdiskPartition, len(pt.Partitions))
	for idx, p := range pt.Partitions {
		partitions[idx] = SgdiskPartition{
			Bootable: p.Bootable,
			Start:    pt.BytesToSectors(p.Start),
			Size:     pt.BytesToSectors(p.Size),
			Type:     p.Type,
			Name:     p.Label,
			Attrs:    p.Attrs,
		}

		if p.UUID != "" {
			u := uuid.MustParse(p.UUID)
			partitions[idx].UUID = &u
		}
	}

	stageOptions := &SgdiskStageOptions{
		UUID:       uuid.MustParse(pt.UUID),
		Partitions: partitions,
	}

	return stageOptions
}

type PartTool string

const (
	PTSfdisk PartTool = "sfdisk"
	PTSgdisk PartTool = "sgdisk"
)

func GenImagePrepareStages(pt *disk.PartitionTable, filename string, partTool PartTool, sourcePipeline string) []*Stage {
	stages := make([]*Stage, 0)

	// create an empty file of the given size via `org.osbuild.truncate`
	stage := NewTruncateStage(
		&TruncateStageOptions{
			Filename: filename,
			Size:     fmt.Sprintf("%d", pt.Size),
		})

	stages = append(stages, stage)

	// create the partition layout in the empty file
	loopback := NewLoopbackDevice(
		&LoopbackDeviceOptions{
			Filename: filename,
			Lock:     true,
		},
	)

	switch partTool {
	case PTSfdisk:
		sfOptions := sfdiskStageOptions(pt)
		sfdisk := NewSfdiskStage(sfOptions, loopback)
		stages = append(stages, sfdisk)
	case PTSgdisk:
		sgOptions := sgdiskStageOptions(pt)
		sgdisk := NewSgdiskStage(sgOptions, loopback)
		stages = append(stages, sgdisk)
	default:
		panic("programming error: unknown PartTool: " + partTool)
	}

	// Generate all the needed "devices", like LUKS2 and LVM2
	s := GenDeviceCreationStages(pt, filename)
	stages = append(stages, s...)

	// Generate all the filesystems, subvolumes, and swap areas on partitons
	// and devices
	s = GenFsStages(pt, filename, sourcePipeline)
	stages = append(stages, s...)

	return stages
}

func GenImageFinishStages(pt *disk.PartitionTable, filename string) []*Stage {
	return GenDeviceFinishStages(pt, filename)
}

func GenImageKernelOptions(pt *disk.PartitionTable, mountConfiguration MountConfiguration) (string, []string, error) {
	cmdline := make([]string, 0)

	rootFs := pt.FindMountable("/")
	if rootFs == nil {
		return "", nil, fmt.Errorf("root filesystem must be defined for kernel-cmdline stage, this is a programming error")
	}
	rootFsUUID := rootFs.GetFSSpec().UUID

	// if /usr is on a separate filesystem, it needs to be defined in the
	// kernel cmdline options for autodiscovery (when there's no /etc/fstab)
	// see:
	//  - https://github.com/systemd/systemd/issues/24027
	//  - https://github.com/systemd/systemd/pull/33397
	if usrFs := pt.FindMountable("/usr"); usrFs != nil && mountConfiguration != MOUNT_CONFIGURATION_FSTAB {
		fsOptions, err := usrFs.GetFSTabOptions()
		if err != nil {
			panic(fmt.Sprintf("error getting filesystem options for /usr mountpoint: %s", err))
		}
		cmdline = append(
			cmdline,
			fmt.Sprintf("mount.usr=UUID=%s", usrFs.GetFSSpec().UUID),
			fmt.Sprintf("mount.usrfstype=%s", usrFs.GetFSType()),
			fmt.Sprintf("mount.usrflags=%s", fsOptions.MntOps),
		)
	}

	genOptions := func(e disk.Entity, path []disk.Entity) error {
		switch ent := e.(type) {
		case *disk.LUKSContainer:
			karg := "luks.uuid=" + ent.UUID
			cmdline = append(cmdline, karg)
		case *disk.BtrfsSubvolume:
			if ent.Mountpoint == "/" && mountConfiguration != MOUNT_CONFIGURATION_UNITS {
				// if we're using mount units, the rootflags will be added
				// separately (below)
				karg := "rootflags=subvol=" + ent.Name
				cmdline = append(cmdline, karg)
			}
		}
		return nil
	}

	if mountConfiguration == MOUNT_CONFIGURATION_UNITS {
		// The systemd-remount-fs service reads /etc/fstab to discover mount
		// options for / and /usr. Without an /etc/fstab, / and /usr do not get
		// remounted, which means if they are mounted read-only in the initrd,
		// they will remain read-only. Flip the option if we're using only
		// mount units, otherwise the filesystems will stay mounted 'ro'.
		//
		// See https://www.freedesktop.org/software/systemd/man/latest/systemd-remount-fs.service.html
		for idx := range cmdline {
			// TODO: consider removing 'ro' from static image configurations
			// and adding either 'ro' or 'rw' here based on the value of
			// mountUnits.
			if cmdline[idx] == "ro" {
				cmdline[idx] = "rw"
				break
			}
		}

		// set the rootflags for the same reason as above
		fsOptions, err := rootFs.GetFSTabOptions()
		if err != nil {
			panic(fmt.Sprintf("error getting filesystem options for / mountpoint: %s", err))
		}

		// if the options are just 'defaults', there's no need to add rootflags
		if fsOptions.MntOps != "defaults" {
			cmdline = append(cmdline, fmt.Sprintf("rootflags=%s", fsOptions.MntOps))
		}
	}

	_ = pt.ForEachEntity(genOptions)
	return rootFsUUID, cmdline, nil
}
