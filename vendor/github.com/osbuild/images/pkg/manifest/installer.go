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

	// If set, the kickstart file will be added to the bootiso-tree at the
	// default path for osbuild, otherwise any kickstart options will be
	// configured in the default location for interactive defaults in the
	// rootfs. Enabling UnattendedKickstart automatically enables this option
	// because automatic installations cannot be configured using interactive
	// defaults.
	ISORootKickstart bool
}
