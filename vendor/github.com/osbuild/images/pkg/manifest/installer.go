package manifest

// Contains all configuration applied to installer type images such as
// Anaconda or CoreOS installer ones.
type InstallerCustomizations struct {
	FIPS bool

	KernelOptionsAppend []string

	EnabledAnacondaModules  []string
	DisabledAnacondaModules []string

	AdditionalDracutModules []string
	AdditionalDrivers       []string

	// Uses the old, deprecated, Anaconda config option "kickstart-modules".
	// Only for RHEL 8.
	UseLegacyAnacondaConfig bool

	LoraxTemplates       []InstallerLoraxTemplate // Templates to run with org.osbuild.lorax
	LoraxTemplatePackage string                   // Package containing lorax templates, added to build pipeline
	LoraxLogosPackage    string                   // eg. fedora-logos, fedora-eln-logos, redhat-logos
	LoraxReleasePackage  string                   // eg. fedora-release, fedora-release-eln, redhat-release

	// ISOFiles contains files to copy from the `anaconda-tree` to the ISO root, this is
	// used to copy (for example) license and legal information into the root of the ISO. An
	// array of source (in anaconda-tree) and destination (in iso-tree).
	ISOFiles [][2]string

	// Install weak dependencies in the installer environment
	InstallWeakDeps bool

	DefaultMenu int

	Product   string
	Variant   string
	OSVersion string
	Release   string
	Preview   bool
}

type InstallerLoraxTemplate struct {
	Path string `yaml:"path"`
	// Should this template be executed after dracut? Defaults to not.
	AfterDracut bool `yaml:"after_dracut,omitempty"`
}
