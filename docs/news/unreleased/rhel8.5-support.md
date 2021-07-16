# Add support for RHEL 8.5 main image types

OSBuild Composer can now build RHEL 8.5 images.  The following new image types
are supported: qcow2, vhd, vmdk, openstack, and ami

To support these image types, the following new types were added to support the
functionality in osbuild.

Stages:
- org.osbuild.copy
- org.osbuild.truncate
- org.osbuild.sfdisk
- org.osbuild.qemu
- org.osbuild.mkfs.btrfs
- org.osbuild.mkfs.ext4
- org.osbuild.mkfs.fat
- org.osbuild.mkfs.xfs
- org.osbuild.grub2.inst


Devices:
- org.osbuild.loopback

Mounts:
- org.osbuild.btrfs
- org.osbuild.ext4
- org.osbuild.fat
- org.osbuild.xfs
