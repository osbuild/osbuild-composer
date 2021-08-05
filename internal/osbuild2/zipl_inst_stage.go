package osbuild2

// Install the Z Initial Program Loader

type ZiplInstStageOptions struct {
	Kernel string `json:"kernel"`

	// The offset of the partition containing /boot
	Location uint64 `json:"location"`

	SectorSize *uint64 `json:"sector-size,omitempty"`
}

func (ZiplInstStageOptions) isStageOptions() {}

// Return a new zipl.inst stage. The 'disk' parameter must represent the
// (entire) device that contains the /boot partition.
func NewZiplInstStage(options *ZiplInstStageOptions, disk *Device, devices *Devices, mounts *Mounts) *Stage {
	// create a new devices map and add the disk to it
	devmap := map[string]Device(*devices)
	devmap["disk"] = *disk
	ziplDevices := Devices(devmap)
	return &Stage{
		Type:    "org.osbuild.zipl.inst",
		Options: options,
		Devices: ziplDevices,
		Mounts:  *mounts,
	}
}
