package osbuild

import (
	"fmt"

	"github.com/osbuild/images/pkg/disk"
)

// Partition a target using sfdisk(8)

type SfdiskStageOptions struct {
	// The type of the partition table
	Label string `json:"label"`

	// UUID for the disk image's partition table
	UUID string `json:"uuid"`

	// Partition layout
	Partitions []SfdiskPartition `json:"partitions,omitempty"`
}

func (SfdiskStageOptions) isStageOptions() {}

// Description of a partition
type SfdiskPartition struct {
	// Mark the partition as bootable (dos)
	Bootable bool `json:"bootable,omitempty"`

	// The partition name (GPT)
	Name string `json:"name,omitempty"`

	// The size of the partition
	Size uint64 `json:"size,omitempty"`

	// The start offset of the partition
	Start uint64 `json:"start,omitempty"`

	// The partition type (UUID or identifier)
	Type string `json:"type,omitempty"`

	// UUID of the partition (GPT)
	UUID string `json:"uuid,omitempty"`

	// Partition attribute flags to set (GPT)
	Attrs []uint `json:"attrs,omitempty"`
}

func (o SfdiskStageOptions) validate() error {
	if o.Label == disk.PT_DOS.String() && len(o.Partitions) > 4 {
		// DOS partition tables only support more than 4 partitions if one of the first
		// 4 (primary) slots is an extended partition. Since the JSON format doesn't
		// explicitly distinguish between primary and logical partitions, we simply
		// verify that an extended partition exists within these primary slots.
		for i := range 4 {
			if o.Partitions[i].Type == disk.ExtendedPartitionDOSID {
				return nil
			}
		}
		return fmt.Errorf("sfdisk stage creation failed: \"dos\" partition table only supports up to 4 partitions: got %d", len(o.Partitions))
	}
	return nil
}

func NewSfdiskStage(options *SfdiskStageOptions, device *Device) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.sfdisk",
		Options: options,
		Devices: map[string]Device{
			"device": *device,
		},
	}
}
