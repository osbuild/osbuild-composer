package osbuild2

func NewBtrfsMount(source, target string) *Mount {
	return &Mount{
		Type:   "org.osbuild.btrfs",
		Source: source,
		Target: target,
	}
}
