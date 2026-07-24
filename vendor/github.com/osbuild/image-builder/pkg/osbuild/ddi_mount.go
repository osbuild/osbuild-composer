package osbuild

type DDIMountOptions struct {
	GrowFS      *bool  `json:"growfs,omitempty"`
	ReadOnly    *bool  `json:"read-only,omitempty"`
	Fsck        *bool  `json:"fsck,omitempty"`
	Discard     string `json:"discard,omitempty"`
	ImagePolicy string `json:"image-policy,omitempty"`
	ImageFilter string `json:"image-filter,omitempty"`
}

func (DDIMountOptions) isMountOptions() {}

func NewDDIMount(name, source, target string) *Mount {
	return &Mount{
		Type:   "org.osbuild.ddi",
		Name:   name,
		Source: source,
		Target: target,
	}
}
