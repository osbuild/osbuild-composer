// nolint: deadcode,unused // Helper functions for future implementations of pipelines
package escl60

// This file defines package sets that are used by more than one image type.

import "github.com/osbuild/osbuild-composer/internal/rpmmd"

// BUILD PACKAGE SETS

// distro-wide build package set
func buildPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"yum", "dosfstools", "e2fsprogs", "glibc",
			//
			// centos 8, require lorax-templates directly
			// "lorax-templates-generic",
			// "lorax-templates-rhel",
			//
			"policycoreutils", "python36",
			"python3-iniparse", "qemu-img", "selinux-policy-targeted", "systemd",
			"tar", "xfsprogs", "xz",
		},
	}
}

func isoBuildPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: append(buildPackageSet(),
			"python-mako", "python-iniparse", "qemu-img",
			"tar", "xfsprogs", "xz", "selinux-policy-targeted", "genisoimage", "isomd5sum",
			"xorriso", "lorax", "squashfs-tools", "grub2-pc-modules", "grub2-tools", "efibootmgr",
			"grub2-tools-minimal", "grub2-tools-extra", "dracut-network", "dracut-config-generic",
			"dracut-fips", "dracut-tools", "kernel-modules-extra", "tmux", //add
			"rng-tools", "net-tools", "NetworkManager", "yum-langpacks"), //add
	}
}
