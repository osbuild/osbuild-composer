package fedora

const VERSION_BRANCHED = "43"
const VERSION_RAWHIDE = "43"

// Fedora 43 and later we reset the machine-id file to align ourselves with the
// other Fedora variants.
const VERSION_FIRSTBOOT = "43"

func VersionReplacements() map[string]string {
	return map[string]string{
		"VERSION_BRANCHED": VERSION_BRANCHED,
		"VERSION_RAWHIDE":  VERSION_RAWHIDE,
	}
}
