package manifest

// Contains all configuration applied to installer type images such as
// Anaconda or CoreOS installer ones.
type InstallerCustomizations struct {
	FIPS bool

	AdditionalKernelOpts []string

	EnabledAnacondaModules  []string
	DisabledAnacondaModules []string

	AdditionalDracutModules []string
	AdditionalDrivers       []string

	// Uses the old, deprecated, Anaconda config option "kickstart-modules".
	// Only for RHEL 8.
	UseLegacyAnacondaConfig bool

	// Temporary
	UseRHELLoraxTemplates bool

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
