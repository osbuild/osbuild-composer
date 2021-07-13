// nolint: deadcode,unused // Helper functions for future implementations of pipelines
package escl60

// This file defines package sets that are used by more than one image type.

import "github.com/osbuild/osbuild-composer/internal/rpmmd"

func x8664IsoBuildPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: append(isoBuildPackageSet(), "shim-x64", "shim-ia32", "grub2-efi-x64",
			"grub2-efi-x64-cdboot", "grub2-efi-ia32-cdboot", "syslinux"),
	}
}
