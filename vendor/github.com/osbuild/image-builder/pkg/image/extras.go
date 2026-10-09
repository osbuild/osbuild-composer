package image

import "fmt"

// SysextPipelineName returns the osbuild pipeline name for a sysext with the
// given name and format.
func SysextPipelineName(name, format string) string {
	return fmt.Sprintf("sysext-%s-%s", name, format)
}

// PartitionPipelineName returns the osbuild pipeline name for a split
// partition with the given name and optional compression.
func PartitionPipelineName(name, compression string) string {
	if compression != "" {
		return fmt.Sprintf("partition-%s-%s", name, compression)
	}
	return "partition-" + name
}

// FilePrepPipelineName returns the osbuild pipeline name for the
// intermediate file-prep pipeline that extracts a file from the disk image.
func FilePrepPipelineName(name string) string {
	return "file-" + name + "-prep"
}

// FilePipelineName returns the osbuild pipeline name for a file extra
// with the given name and optional compression.
func FilePipelineName(name, compression string) string {
	if compression != "" {
		return fmt.Sprintf("file-%s-%s", name, compression)
	}
	return "file-" + name
}
