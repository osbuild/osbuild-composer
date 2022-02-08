package osbuild2

import (
	"fmt"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/disk"
)

type Devices map[string]Device

type Device struct {
	Type    string        `json:"type"`
	Parent  string        `json:"parent,omitempty"`
	Options DeviceOptions `json:"options,omitempty"`
}

type DeviceOptions interface {
	isDeviceOptions()
}

func deviceName(p disk.Entity) string {
	if p == nil {
		panic("device is nil; this is a programming error")
	}

	switch payload := p.(type) {
	case disk.Mountable:
		return pathdot(payload.GetMountpoint())
	case *disk.LUKSContainer:
		return "luks-" + payload.UUID[:4]
	case *disk.LVMVolumeGroup:
		return payload.Name + "vg"
	case *disk.LVMLogicalVolume:
		return payload.Name
	}
	panic(fmt.Sprintf("unsupported device type in deviceName: '%T'", p))
}

func getDevices(path []disk.Entity, filename string) (map[string]Device, string) {
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

func pathdot(path string) string {
	if path == "/" {
		return "root"
	}

	path = strings.TrimLeft(path, "/")

	return strings.ReplaceAll(path, "/", ".")
}
