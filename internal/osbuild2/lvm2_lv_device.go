package osbuild2

// Provide access to LVM2 Logical Volume (LV)

type LVM2LVDeviceOptions struct {
	// Logical volume to activate
	Volume string `json:"volume"`
}

func (LVM2LVDeviceOptions) isDeviceOptions() {}

func NewLVM2LVDevice(options *LoopbackDeviceOptions) *Device {
	return &Device{
		Type:    "org.osbuild.lvm2.lv",
		Options: options,
	}
}
