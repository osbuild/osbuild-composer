package blueprint

import (
	"fmt"
)

type PartitioningMode string

const (
	// AutoLVMPartitioningMode creates a LVM layout if the filesystem
	// contains a mountpoint that's not defined in the base partition table
	// of the specified image type. In the other case, a raw layout is used.
	AutoLVMPartitioningMode PartitioningMode = "auto-lvm"

	// LVMPartitioningMode always creates an LVM layout.
	LVMPartitioningMode PartitioningMode = "lvm"

	// RawPartitioningMode always creates a raw layout.
	RawPartitioningMode PartitioningMode = "raw"

	// BtrfsPartitioningMode creates a btrfs layout.
	BtrfsPartitioningMode PartitioningMode = "btrfs"

	// DefaultPartitioningMode is AutoLVMPartitioningMode and is the empty state
	DefaultPartitioningMode PartitioningMode = ""
)

// GetPartitioningMode converts the string to a disk.PartitioningMode type
func (c *Customizations) GetPartitioningMode() (PartitioningMode, error) {
	if c == nil {
		return DefaultPartitioningMode, nil
	}

	switch c.PartitioningMode {
	case "raw":
		return RawPartitioningMode, nil
	case "lvm":
		return LVMPartitioningMode, nil
	case "auto-lvm":
		return AutoLVMPartitioningMode, nil
	case "":
		return DefaultPartitioningMode, nil
	default:
		return DefaultPartitioningMode, fmt.Errorf("invalid partitioning mode '%s'", c.PartitioningMode)
	}
}
