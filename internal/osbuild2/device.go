package osbuild2

type Devices map[string]Device

type Device struct {
	Type    string        `json:"type"`
	Options DeviceOptions `json:"options,omitempty"`
}

type DeviceOptions interface {
	isDeviceOptions()
}
