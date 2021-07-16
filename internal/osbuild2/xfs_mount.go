package osbuild2

func NewXfsMount(source, target string) *Mount {
	return &Mount{
		Type:   "org.osbuild.xfs",
		Source: source,
		Target: target,
	}
}
