package osbuild2

type Devices interface {
	isStageDevices()
}

type Device struct {
	Type    string        `json:"type"`
	Options DeviceOptions `json:"options,omitempty"`
}

type DeviceOptions interface {
	isDeviceOptions()
}
