// Package bootc handles resolving information from bootc-based containers for
// generating manifests for bootc-derived images.
package bootc

import "github.com/osbuild/image-builder/pkg/bib/osinfo"

// ContainerType is the boot artifact type reported by bootc container inspect.
type ContainerType string

const (
	ContainerTypeAboot    ContainerType = "aboot"
	ContainerTypeAbootEFI ContainerType = "aboot-efi"
)

// IsAboot reports whether the type describes an aboot partition payload.
func (t ContainerType) IsAboot() bool {
	return t == ContainerTypeAboot || t == ContainerTypeAbootEFI
}

// IsAbootEFI reports whether the aboot payload requires an ESP for ukiboot.
func (t ContainerType) IsAbootEFI() bool {
	return t == ContainerTypeAbootEFI
}

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

	// Is the container using a unified kernel?
	UnifiedKernel bool

	// The boot artifact type reported by the container.
	ContainerType ContainerType

	// What bootloader should be passed?
	Bootloader *string
}
