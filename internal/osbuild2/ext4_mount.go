package osbuild2

func NewExt4Mount(source, target string) *Mount {
	return &Mount{
		Type:   "org.osbuild.ext4",
		Source: source,
		Target: target,
	}
}
