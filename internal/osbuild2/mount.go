package osbuild2

type Mounts []Mount

type Mount struct {
	Name    string        `json:"name"`
	Type    string        `json:"type"`
	Source  string        `json:"source"`
	Target  string        `json:"target"`
	Options *MountOptions `json:"options,omitempty"`
}

type MountOptions interface {
	isMountOptions()
}
