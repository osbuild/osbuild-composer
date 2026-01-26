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

	// InstallWeakDeps determines if weak dependencies are installed in the installer
	// environment.
	InstallWeakDeps *bool `yaml:"install_weak_deps,omitempty"`

	// Lorax template settings for org.osbuild.lorax stage
	LoraxTemplates       []manifest.InstallerLoraxTemplate `yaml:"lorax_templates,omitempty"`
	LoraxTemplatePackage *string                           `yaml:"lorax_template_package"`
	LoraxLogosPackage    *string                           `yaml:"lorax_logos_package"`
	LoraxReleasePackage  *string                           `yaml:"lorax_release_package"`

	// ISOFiles contains files to copy from the `anaconda-tree` to the ISO root, this is
	// used to copy (for example) license and legal information into the root of the ISO. An
	// array of source (in anaconda-tree) and destination (in iso-tree).
	ISOFiles [][2]string `yaml:"iso_files"`
}

// InheritFrom inherits unset values from the provided parent configuration and
// returns a new structure instance, which is a result of the inheritance.
func (c *InstallerConfig) InheritFrom(parentConfig *InstallerConfig) *InstallerConfig {
	return shallowMerge(c, parentConfig)
}
