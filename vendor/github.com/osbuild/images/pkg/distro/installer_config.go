package distro

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
	ISORootKickstart                   *bool    `yaml:"iso_root_kickstart"`

	// SquashfsRootfs will set SquashfsRootfs as rootfs in the iso image
	SquashfsRootfs *bool `yaml:"squashfs_rootfs"`
}

// InheritFrom inherits unset values from the provided parent configuration and
// returns a new structure instance, which is a result of the inheritance.
func (c *InstallerConfig) InheritFrom(parentConfig *InstallerConfig) *InstallerConfig {
	return shallowMerge(c, parentConfig)
}
