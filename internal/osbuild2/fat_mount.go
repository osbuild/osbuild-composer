package osbuild2

func NewFATMount(source, target string) *Mount {
	return &Mount{
		Type:   "org.osbuild.fat",
		Source: source,
		Target: target,
	}
}
