package distro

// Tweaks is used to set small config changes and workarounds that are not
// specific image definition configurations but are required for builds to
// work.
type Tweaks struct {
	RPMKeys *RPMKeysTweaks `yaml:"rpmkeys"`
}

type RPMKeysTweaks struct {
	// Path to rpmkeys binary to use for importing and verifying GPG rpm signatures.
	BinPath string `yaml:"binary_path"`

	// IgnoreBuildImportFailures enables rpmkeys.ignore_import_failures for the
	// build pipeline's rpm stage. This is needed when building on a host
	// distro that does not support the format of one or more keys used by the
	// target distro's repository configs.
	IgnoreBuildImportFailures bool `yaml:"ignore_build_import_failures"`
}

func (t *Tweaks) InheritFrom(parentConfig *Tweaks) *Tweaks {
	if t == nil {
		t = &Tweaks{}
	}
	return shallowMerge(t, parentConfig)
}
