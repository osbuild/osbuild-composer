package distro

import (
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
)

// ISOConfig represents configuration for the ISO part of images that are packed
// into ISOs.
type ISOConfig struct {
	// BootType defines what type of bootloader is used for the iso
	BootType *manifest.ISOBootType `yaml:"boot_type,omitempty"`

	// RootfsType defines what rootfs (squashfs, erofs,ext4)
	// is used
	RootfsType *manifest.ISORootfsType `yaml:"rootfs_type,omitempty"`

	// set only when RootfsType is erofs
	ErofsOptions *osbuild.ErofsStageOptions `yaml:"erofs_options,omitempty"`

	// Metadata field on the ISO for the volume id
	Label *string `yaml:"label,omitempty"`

	// Metadata field on the ISO for the creation tool
	Preparer *string `yaml:"preparer,omitempty"`

	// Metadata field on the ISO for the publisher
	Publisher *string `yaml:"publisher,omitempty"`

	// Metadata field on the ISO for the application ID
	Application *string `yaml:"application,omitempty"`
}

// InheritFrom inherits unset values from the provided parent configuration and
// returns a new structure instance, which is a result of the inheritance.
func (c *ISOConfig) InheritFrom(parentConfig *ISOConfig) *ISOConfig {
	return shallowMerge(c, parentConfig)
}
