package osbuild

import (
	"regexp"
	"strings"
)

// vfat volume-id is a 32-bit hex number (mkfs.vfat(8))
const fatVolIDRegex = `^[a-fA-F0-9]{8}$`

type MkfsFATStageOptions struct {
	VolID   string `json:"volid"`
	Label   string `json:"label,omitempty"`
	FATSize *int   `json:"fat-size,omitempty"`
}

func (MkfsFATStageOptions) isStageOptions() {}

func NewMkfsFATStage(options *MkfsFATStageOptions, devices map[string]Device) *Stage {
	return &Stage{
		Type:    "org.osbuild.mkfs.fat",
		Options: options,
		Devices: devices,
	}
}

func isFATVolID(id string) bool {
	// Internally, we generate FAT volume IDs with a dash (-) in the middle.
	// This is also how they're represented by udev (in /dev/disk/by-uuid). The
	// mkfs.vfat command doesn't accept dashes, which is why we remove them
	// when generating the mkfs stage in mkfs_stage.go. This check removes all
	// dashes to determine if the given id is a valid vfat volid.
	volidre := regexp.MustCompile(fatVolIDRegex)
	return volidre.MatchString(strings.ReplaceAll(id, "-", ""))
}
