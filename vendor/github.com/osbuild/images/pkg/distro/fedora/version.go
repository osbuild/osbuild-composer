package fedora

const VERSION_BRANCHED = "42"
const VERSION_RAWHIDE = "42"

// Fedora version 41 and later use a plain squashfs rootfs on the iso instead of
// compressing an ext4 filesystem.
const VERSION_ROOTFS_SQUASHFS = "41"

// Fedora 43 and later we reset the machine-id file to align ourselves with the
// other Fedora variants.
const VERSION_FIRSTBOOT = "43"

// Version at which we stop installing weak dependencies for Fedora Minimal
const VERSION_MINIMAL_WEAKDEPS = "43"

func VersionReplacements() map[string]string {
	return map[string]string{
		"VERSION_BRANCHED":        VERSION_BRANCHED,
		"VERSION_RAWHIDE":         VERSION_RAWHIDE,
		"VERSION_ROOTFS_SQUASHFS": VERSION_ROOTFS_SQUASHFS,
	}
}
