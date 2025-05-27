package distro

// InstallerConfig represents a configuration for the installer
// part of an Installer image type.
type InstallerConfig struct {
	// Additional dracut modules and drivers to enable
	AdditionalDracutModules []string `yaml:"additional_dracut_modules"`
	AdditionalDrivers       []string `yaml:"additional_drivers"`
}
