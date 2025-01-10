package fedora

const VERSION_BRANCHED = "42"
const VERSION_RAWHIDE = "42"

// Fedora version 41 and later use a plain squashfs rootfs on the iso instead of
// compressing an ext4 filesystem.
const VERSION_ROOTFS_SQUASHFS = "41"
