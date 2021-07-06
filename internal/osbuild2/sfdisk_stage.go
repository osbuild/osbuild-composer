package osbuild2

// Partition a target using sfdisk(8)

type SfdiskStageOptions struct {
	// The type of the partition table
	Label string `json:"label"`

	// UUID for the disk image's partition table
	UUID string `json:"uuid"`

	// Partition layout
	Partitions []Partition `json:"partitions,omitempty"`
}

func (SfdiskStageOptions) isStageOptions() {}

// Description of a partition
type Partition struct {
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
}

type SfdiskStageDevices struct {
	Device Device `json:"device"`
}

func (SfdiskStageDevices) isStageDevices() {}

func NewSfdiskStage(options *SfdiskStageOptions, devices *SfdiskStageDevices) *Stage {
	return &Stage{
		Type:    "org.osbuild.sfdisk",
		Options: options,
		Devices: devices,
	}
}
