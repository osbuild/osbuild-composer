package distro

import (
	"github.com/osbuild/images/pkg/manifest"
)

// InstallerConfig represents a configuration for the installer
// part of an Installer image type.
type InstallerConfig struct {
	EnabledAnacondaModules []string `yaml:"enabled_anaconda_modules"`

	// Additional dracut modules and drivers to enable
	AdditionalDracutModules []string `yaml:"additional_dracut_modules"`
	AdditionalDrivers       []string `yaml:"additional_drivers"`

	// XXX: this is really here only for compatibility/because of drift in the "imageInstallerImage"
	// between fedora/rhel
	KickstartUnattendedExtraKernelOpts []string `yaml:"kickstart_unattended_extra_kernel_opts"`

	// DefaultMenu will set the grub2 iso menu's default setting
	DefaultMenu *int `yaml:"default_menu"`

	// RootfsType defines what rootfs (squashfs, erofs,ext4)
	// is used
	ISORootfsType *manifest.ISORootfsType `yaml:"iso_rootfs_type,omitempty"`

	// BootType defines what type of bootloader is used for the iso
	ISOBootType *manifest.ISOBootType `yaml:"iso_boot_type,omitempty"`
}

// InheritFrom inherits unset values from the provided parent configuration and
// returns a new structure instance, which is a result of the inheritance.
func (c *InstallerConfig) InheritFrom(parentConfig *InstallerConfig) *InstallerConfig {
	return shallowMerge(c, parentConfig)
}
