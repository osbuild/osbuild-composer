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

	ISORootfsType ISORootfsType
	ISOBoot       ISOBootType

	DefaultMenu int

	ISOLabel  string
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
