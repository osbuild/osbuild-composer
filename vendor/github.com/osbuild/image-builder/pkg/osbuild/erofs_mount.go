package osbuild

func NewErofsMount(name, source, target string) *Mount {
	return &Mount{
		Type:   "org.osbuild.erofs",
		Name:   name,
		Source: source,
		Target: target,
	}
}
