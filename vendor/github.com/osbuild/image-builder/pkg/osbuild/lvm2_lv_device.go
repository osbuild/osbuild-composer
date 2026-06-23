package osbuild

// Provide access to LVM2 Logical Volume (LV)

type LVM2LVDeviceOptions struct {
	// Logical volume to activate
	Volume string `json:"volume"`
	// The partition the volume group is located on
	VGPartnum *int `json:"vg_partnum,omitempty"`
}

func (LVM2LVDeviceOptions) isDeviceOptions() {}

func NewLVM2LVDevice(parent string, options *LVM2LVDeviceOptions) *Device {
	return &Device{
		Type:    "org.osbuild.lvm2.lv",
		Parent:  parent,
		Options: options,
	}
}
