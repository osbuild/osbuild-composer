package osbuild2

type Mounts interface {
	isStageMounts()
}

type Mount struct {
	Type    string        `json:"type"`
	Source  string        `json:"source"`
	Target  string        `json:"target"`
	Options *MountOptions `json:"options,omitempty"`
}

type MountOptions interface {
	isMountOptions()
}
