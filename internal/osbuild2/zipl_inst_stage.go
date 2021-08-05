package osbuild2

// Install the Z Initial Program Loader

type ZiplInstStageOptions struct {
	Kernel string `json:"kernel"`

	// The offset of the partition containing /boot
	Location uint64 `json:"location"`

	SectorSize *uint64 `json:"sector-size,omitempty"`
}

func (ZiplInstStageOptions) isStageOptions() {}

type ZiplInstStageDevices map[string]Device

func (ZiplInstStageDevices) isStageDevices() {}

// Return a new zipl.inst stage. A device needs to be specified as 'disk' and root mountpoint must be provided
func NewZiplInstStage(options *ZiplInstStageOptions, devices *CopyStageDevices, mounts *Mounts) *Stage {
	return &Stage{
		Type:    "org.osbuild.zipl.inst",
		Options: options,
		Devices: devices,
		Mounts:  *mounts,
	}
}
