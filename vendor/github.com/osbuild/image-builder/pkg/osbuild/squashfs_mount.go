package osbuild

func NewSquashfsMount(name, source, target string) *Mount {
	return &Mount{
		Type:   "org.osbuild.squashfs",
		Name:   name,
		Source: source,
		Target: target,
	}
}
