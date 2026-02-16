// Package bootc handles resolving information from bootc-based containers for
// generating manifests for bootc-derived images.
package bootc

import "github.com/osbuild/images/pkg/bib/osinfo"

// Info contains all the information from the bootc container that is
// required to create a manifest for a bootc-based image.
type Info struct {
	// The name of the container image that generated the info
	Imgref string

	// The container image ID
	ImageID string

	// Information related to the OS in the container
	OSInfo *osinfo.Info

	// The container's hardware architecture
	Arch string

	// The default root filesystem from the container's bootc config
	DefaultRootFs string

	// The size of the container image
	Size uint64
}
