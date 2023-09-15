package rhel8

// This file defines package sets that are used by more than one image type.

import (
	"fmt"

	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

// installer boot package sets, needed for booting and
// also in the build host

func anacondaBootPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{}

	grubCommon := rpmmd.PackageSet{
		Include: []string{
			"grub2-tools",
			"grub2-tools-extra",
			"grub2-tools-minimal",
		},
	}

	efiCommon := rpmmd.PackageSet{
		Include: []string{
			"efibootmgr",
		},
	}

	switch t.arch.Name() {
	case platform.ARCH_X86_64.String():
		ps = ps.Append(grubCommon)
		ps = ps.Append(efiCommon)
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"grub2-efi-ia32-cdboot",
				"grub2-efi-x64",
				"grub2-efi-x64-cdboot",
				"grub2-pc",
				"grub2-pc-modules",
				"shim-ia32",
				"shim-x64",
				"syslinux",
				"syslinux-nonlinux",
			},
		})
	case platform.ARCH_AARCH64.String():
		ps = ps.Append(grubCommon)
		ps = ps.Append(efiCommon)
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"grub2-efi-aa64-cdboot",
				"grub2-efi-aa64",
				"shim-aa64",
			},
		})

	default:
		panic(fmt.Sprintf("unsupported arch: %s", t.arch.Name()))
	}

	return ps
}

// packages that are only in some (sub)-distributions
func distroSpecificPackageSet(t *imageType) rpmmd.PackageSet {
	if t.arch.distro.isRHEL() {
		return rpmmd.PackageSet{
			Include: []string{"insights-client"},
		}
	}
	return rpmmd.PackageSet{}
}

func minimalrpmPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core",
			"initial-setup",
			"libxkbcommon",
			"NetworkManager-wifi",
			"iwl7260-firmware",
			"iwl3160-firmware",
		},
	}
}
