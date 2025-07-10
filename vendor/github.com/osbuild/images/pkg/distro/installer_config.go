package distro

// InstallerConfig represents a configuration for the installer
// part of an Installer image type.
type InstallerConfig struct {
	// Additional dracut modules and drivers to enable
	AdditionalDracutModules   []string `yaml:"additional_dracut_modules"`
	AdditionalDrivers         []string `yaml:"additional_drivers"`
	AdditionalAnacondaModules []string `yaml:"additional_anaconda_modules"`

	// SquashfsRootfs will set SquashfsRootfs as rootfs in the iso image
	SquashfsRootfs *bool `yaml:"squashfs_rootfs"`
}
