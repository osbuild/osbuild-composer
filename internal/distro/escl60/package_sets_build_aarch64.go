// nolint: deadcode,unused // Helper functions for future implementations of pipelines
package escl60

// This file defines package sets that are used by more than one image type.

import "github.com/osbuild/osbuild-composer/internal/rpmmd"

func aarch64IsoBuildPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: append(isoBuildPackageSet(), "shim-aa64", "grub2-efi-aa64",
			"grub2-efi-aa64-cdboot"),
	}
}
