package fedora

const VERSION_BRANCHED = "42"
const VERSION_RAWHIDE = "42"

// Fedora version 41 and later use a plain squashfs rootfs on the iso instead of
// compressing an ext4 filesystem.
const VERSION_ROOTFS_SQUASHFS = "41"

func VersionReplacements() map[string]string {
	return map[string]string{
		"VERSION_BRANCHED":        VERSION_BRANCHED,
		"VERSION_RAWHIDE":         VERSION_RAWHIDE,
		"VERSION_ROOTFS_SQUASHFS": VERSION_ROOTFS_SQUASHFS,
	}
}
