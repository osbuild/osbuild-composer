package distro

import (
	"github.com/osbuild/images/pkg/osbuild"
)

// DiskConfig represents configuration for the Disk part of images that are packed
// into Disks.
type DiskConfig struct {
	// MountConfiguration determines the mounting system used by the image. For
	// example systemd .mount units to describe the filesystem instead of writing
	// to /etc/fstab or none
	MountConfiguration *osbuild.MountConfiguration `yaml:"mount_configuration,omitempty"`

	// Mostly for RHEL7 compat though might be purposed in the future
	PartitioningTool *osbuild.PartTool `yaml:"partitioning_tool,omitempty"`
}

// InheritFrom inherits unset values from the provided parent configuration and
// returns a new structure instance, which is a result of the inheritance.
func (c *DiskConfig) InheritFrom(parentConfig *DiskConfig) *DiskConfig {
	return shallowMerge(c, parentConfig)
}
