package osbuild

type btrfsMountOptions struct {
	Subvol   string `json:"subvol,omitempty"`
	Compress string `json:"compress,omitempty"`
}

func (b btrfsMountOptions) isMountOptions() {}

func NewBtrfsMount(name, source, target, subvol, compress string) *Mount {
	return &Mount{
		Type:   "org.osbuild.btrfs",
		Name:   name,
		Source: source,
		Target: target,
		Options: btrfsMountOptions{
			Subvol:   subvol,
			Compress: compress,
		},
	}
}
