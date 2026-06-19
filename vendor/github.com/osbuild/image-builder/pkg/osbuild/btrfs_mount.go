package osbuild

type BtrfsMountOptions struct {
	Subvol   string `json:"subvol,omitempty"`
	Compress string `json:"compress,omitempty"`
}

func (b BtrfsMountOptions) isMountOptions() {}

func NewBtrfsMount(name, source, target, subvol, compress string) *Mount {
	return &Mount{
		Type:   "org.osbuild.btrfs",
		Name:   name,
		Source: source,
		Target: target,
		Options: BtrfsMountOptions{
			Subvol:   subvol,
			Compress: compress,
		},
	}
}
