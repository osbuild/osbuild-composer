package osbuild

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/osbuild/images/pkg/disk"
)

type Device struct {
	Type    string        `json:"type"`
	Parent  string        `json:"parent,omitempty"`
	Options DeviceOptions `json:"options,omitempty"`
}

type DeviceOptions interface {
	isDeviceOptions()
}

func GenDeviceCreationStages(pt *disk.PartitionTable, filename string) []*Stage {
	stages := make([]*Stage, 0)

	genStages := func(e disk.Entity, path []disk.Entity) error {

		switch ent := e.(type) {
		case *disk.LUKSContainer:
			// do not include us when getting the devices
			stageDevices, lastName := getDevices(path[:len(path)-1], filename, true)

			// "org.osbuild.luks2.format" expects a "device" to create the VG on,
			// thus rename the last device to "device"
			lastDevice := stageDevices[lastName]
			delete(stageDevices, lastName)
			stageDevices["device"] = lastDevice

			stage := NewLUKS2CreateStage(
				&LUKS2CreateStageOptions{
					UUID:       ent.UUID,
					Passphrase: ent.Passphrase,
					Cipher:     ent.Cipher,
					Label:      ent.Label,
					Subsystem:  ent.Subsystem,
					SectorSize: ent.SectorSize,
					PBKDF: Argon2id{
						Method:      "argon2id",
						Iterations:  ent.PBKDF.Iterations,
						Memory:      ent.PBKDF.Memory,
						Parallelism: ent.PBKDF.Parallelism,
					},
				},
				stageDevices)

			stages = append(stages, stage)

			if ent.Clevis != nil {
				stages = append(stages, NewClevisLuksBindStage(&ClevisLuksBindStageOptions{
					Passphrase: ent.Passphrase,
					Pin:        ent.Clevis.Pin,
					Policy:     ent.Clevis.Policy,
				}, stageDevices))
			}

		case *disk.LVMVolumeGroup:
			// do not include us when getting the devices
			stageDevices, lastName := getDevices(path[:len(path)-1], filename, true)

			// "org.osbuild.lvm2.create" expects a "device" to create the VG on,
			// thus rename the last device to "device"
			lastDevice := stageDevices[lastName]
			delete(stageDevices, lastName)
			stageDevices["device"] = lastDevice

			volumes := make([]LogicalVolume, len(ent.LogicalVolumes))
			for idx, lv := range ent.LogicalVolumes {
				volumes[idx].Name = lv.Name
				// NB: we need to specify the size in bytes, since lvcreate
				// defaults to megabytes
				volumes[idx].Size = fmt.Sprintf("%dB", lv.Size)
			}

			stage := NewLVM2CreateStage(
				&LVM2CreateStageOptions{
					Volumes: volumes,
				}, stageDevices)

			stages = append(stages, stage)
		}

		return nil
	}

	_ = pt.ForEachEntity(genStages)
	return stages
}

func GenDeviceFinishStages(pt *disk.PartitionTable, filename string) []*Stage {
	stages := make([]*Stage, 0)
	removeKeyStages := make([]*Stage, 0)

	genStages := func(e disk.Entity, path []disk.Entity) error {

		switch ent := e.(type) {
		case *disk.LUKSContainer:
			// do not include us when getting the devices
			stageDevices, lastName := getDevices(path[:len(path)-1], filename, true)

			lastDevice := stageDevices[lastName]
			delete(stageDevices, lastName)
			stageDevices["device"] = lastDevice

			if ent.Clevis != nil {
				if ent.Clevis.RemovePassphrase {
					removeKeyStages = append(removeKeyStages, NewLUKS2RemoveKeyStage(&LUKS2RemoveKeyStageOptions{
						Passphrase: ent.Passphrase,
					}, stageDevices))
				}
			}
		case *disk.LVMVolumeGroup:
			// do not include us when getting the devices
			stageDevices, lastName := getDevices(path[:len(path)-1], filename, true)

			// "org.osbuild.lvm2.metadata" expects a "device" to rename the VG,
			// thus rename the last device to "device"
			lastDevice := stageDevices[lastName]
			delete(stageDevices, lastName)
			stageDevices["device"] = lastDevice

			stage := NewLVM2MetadataStage(
				&LVM2MetadataStageOptions{
					VGName: ent.Name,
				}, stageDevices)

			stages = append(stages, stage)
		}

		return nil
	}

	_ = pt.ForEachEntity(genStages)
	// Ensure that "org.osbuild.luks2.remove-key" stages are done after
	// "org.osbuild.lvm2.metadata" stages, we cannot open a device if its
	// password has changed
	stages = append(stages, removeKeyStages...)
	return stages
}

func deviceName(p disk.Entity) string {
	if p == nil {
		panic("device is nil; this is a programming error")
	}

	switch payload := p.(type) {
	case disk.Mountable:
		return pathEscape(payload.GetMountpoint())
	case *disk.LUKSContainer:
		return "luks-" + payload.UUID[:4]
	case *disk.LVMVolumeGroup:
		return payload.Name
	case *disk.LVMLogicalVolume:
		return payload.Name
	case *disk.Btrfs:
		return "btrfs-" + payload.UUID[:4]
	}
	panic(fmt.Sprintf("unsupported device type in deviceName: '%T'", p))
}

// getDevices takes an entity path, and returns osbuild devices required before being able to mount the leaf Mountable
//
// - path is an entity path as defined by the disk.entityPath function
// - filename is the name of an underlying image file (which will get loop-mounted)
// - lockLoopback determines whether the loop device will get locked after creation
//
// The device names are created from the payload that they are holding. This is useful to easily visually map e.g.
// a loopback device and its mount (in the case of ordinary partitions): they should have the same, or similar name.
//
// The first returned value is a map of devices for the given path.
// The second returned value is the name of the last device in the path. This is the device that should be used as the
// source for the mount.
func getDevices(path []disk.Entity, filename string, lockLoopback bool) (map[string]Device, string) {
	var pt *disk.PartitionTable

	do := make(map[string]Device)
	parent := ""
	for _, elem := range path {
		switch e := elem.(type) {
		case *disk.PartitionTable:
			pt = e
		case *disk.Partition:
			if pt == nil {
				panic("path does not contain partition table; this is a programming error")
			}
			lbopt := LoopbackDeviceOptions{
				Filename:   filename,
				Start:      pt.BytesToSectors(e.Start),
				Size:       pt.BytesToSectors(e.Size),
				SectorSize: nil,
				Lock:       lockLoopback,
			}
			name := deviceName(e.Payload)
			do[name] = *NewLoopbackDevice(&lbopt)
			parent = name
		case *disk.LUKSContainer:
			lo := LUKS2DeviceOptions{
				Passphrase: e.Passphrase,
			}
			name := deviceName(e.Payload)
			do[name] = *NewLUKS2Device(parent, &lo)
			parent = name
		case *disk.LVMLogicalVolume:
			lo := LVM2LVDeviceOptions{
				Volume: e.Name,
			}
			name := deviceName(e.Payload)
			do[name] = *NewLVM2LVDevice(parent, &lo)
			parent = name
		}
	}
	return do, parent
}

// pathEscape implements similar path escaping as used by systemd-escape
// https://github.com/systemd/systemd/blob/c57ff6230e4e199d40f35a356e834ba99f3f8420/src/basic/unit-name.c#L389
func pathEscape(path string) string {
	if len(path) == 0 || path == "/" {
		return "-"
	}

	path = strings.Trim(path, "/")

	escapeChars := func(s, char string) string {
		return strings.ReplaceAll(s, char, fmt.Sprintf("\\x%x", char[0]))
	}

	path = escapeChars(path, "\\")
	path = escapeChars(path, "-")

	return strings.ReplaceAll(path, "/", "-")
}

// genOsbuildMount generates an osbuild mount from Mountable mnt
//
// - source is the name of the device that the mount should be created from.
// The name of the mount is derived from the mountpoint of the mountable, escaped with pathEscape. This shouldn't
// create any conflicts, as the mountpoint is unique within the partition table.
func genOsbuildMount(source string, mnt disk.Mountable) (*Mount, error) {
	mountpoint := mnt.GetMountpoint()
	name := pathEscape(mountpoint)
	t := mnt.GetFSType()
	switch t {
	case "xfs":
		return NewXfsMount(name, source, mountpoint), nil
	case "vfat":
		return NewFATMount(name, source, mountpoint), nil
	case "ext4":
		return NewExt4Mount(name, source, mountpoint), nil
	case "btrfs":
		if subvol, isSubvol := mnt.(*disk.BtrfsSubvolume); isSubvol {
			return NewBtrfsMount(name, source, mountpoint, subvol.Name, subvol.Compress), nil
		} else {
			return nil, fmt.Errorf("mounting bare btrfs partition is unsupported: %s", mountpoint)
		}
	default:
		return nil, fmt.Errorf("unknown fs type " + t)
	}
}

// GenMountsDevicesFromPT generates osbuild mounts and devices from a disk.PartitionTable
// filename is the name of the underlying image file (which will get loop-mounted).
//
// Returned values:
// 1) the name of the mount for the filesystem root
// 2) generated mounts
// 3) generated devices
// 4) error if any
func GenMountsDevicesFromPT(filename string, pt *disk.PartitionTable) (string, []Mount, map[string]Device, error) {
	devices := make(map[string]Device, len(pt.Partitions))
	mounts := make([]Mount, 0, len(pt.Partitions))
	var fsRootMntName string
	genMounts := func(mnt disk.Mountable, path []disk.Entity) error {
		stageDevices, leafDeviceName := getDevices(path, filename, false)
		mount, err := genOsbuildMount(leafDeviceName, mnt)
		if err != nil {
			return err
		}

		mountpoint := mnt.GetMountpoint()
		if mountpoint == "/" {
			fsRootMntName = mount.Name
		}

		mounts = append(mounts, *mount)

		// update devices map with new elements from stageDevices
		for devName := range stageDevices {
			if existingDevice, exists := devices[devName]; exists {
				// It is usual that the a device is generated twice for the same Entity e.g. LVM VG, which is OK.
				// Therefore fail only if a device with the same name is generated for two different Entities.
				if !reflect.DeepEqual(existingDevice, stageDevices[devName]) {
					return fmt.Errorf("the device name %q has been generated for two different devices", devName)
				}
			}
			devices[devName] = stageDevices[devName]
		}
		return nil
	}

	if err := pt.ForEachMountable(genMounts); err != nil {
		return "", nil, nil, err
	}

	// sort the mounts, using < should just work because:
	// - a parent directory should be always before its children:
	//   / < /boot
	// - the order of siblings doesn't matter
	sort.Slice(mounts, func(i, j int) bool {
		return mounts[i].Target < mounts[j].Target
	})

	if fsRootMntName == "" {
		return "", nil, nil, fmt.Errorf("no mount found for the filesystem root")
	}

	return fsRootMntName, mounts, devices, nil
}
