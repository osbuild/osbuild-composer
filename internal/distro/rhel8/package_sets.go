package rhel8

// This file defines package sets that are used by more than one image type.

import (
	"fmt"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// BUILD PACKAGE SETS

// distro-wide build package set
func distroBuildPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"dnf",
			"dosfstools",
			"e2fsprogs",
			"glibc",
			"lorax-templates-generic",
			"lorax-templates-rhel",
			"lvm2",
			"policycoreutils",
			"python36",
			"python3-iniparse",
			"qemu-img",
			"selinux-policy-targeted",
			"systemd",
			"tar",
			"xfsprogs",
			"xz",
		},
	}

	switch t.arch.Name() {

	case distro.X86_64ArchName:
		ps = ps.Append(x8664BuildPackageSet(t))

	case distro.Ppc64leArchName:
		ps = ps.Append(ppc64leBuildPackageSet(t))
	}

	return ps
}

// x86_64 build package set
func x8664BuildPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{"grub2-pc"},
	}
}

// ppc64le build package set
func ppc64leBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{"grub2-ppc64le", "grub2-ppc64le-modules"},
	}
}

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
	case distro.X86_64ArchName:
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
	case distro.Aarch64ArchName:
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

// BOOT PACKAGE SETS

func bootPackageSet(t *imageType) rpmmd.PackageSet {
	if !t.bootable {
		return rpmmd.PackageSet{}
	}

	var addLegacyBootPkg bool
	var addUEFIBootPkg bool

	switch bt := t.getBootType(); bt {
	case distro.LegacyBootType:
		addLegacyBootPkg = true
	case distro.UEFIBootType:
		addUEFIBootPkg = true
	case distro.HybridBootType:
		addLegacyBootPkg = true
		addUEFIBootPkg = true
	default:
		panic(fmt.Sprintf("unsupported boot type: %q", bt))
	}

	ps := rpmmd.PackageSet{}

	switch t.arch.Name() {
	case distro.X86_64ArchName:
		if addLegacyBootPkg {
			ps = ps.Append(x8664LegacyBootPackageSet(t))
		}
		if addUEFIBootPkg {
			ps = ps.Append(x8664UEFIBootPackageSet(t))
		}

	case distro.Aarch64ArchName:
		ps = ps.Append(aarch64UEFIBootPackageSet(t))

	case distro.Ppc64leArchName:
		ps = ps.Append(ppc64leLegacyBootPackageSet(t))

	case distro.S390xArchName:
		ps = ps.Append(s390xLegacyBootPackageSet(t))

	default:
		panic(fmt.Sprintf("unsupported boot arch: %s", t.arch.Name()))
	}

	return ps
}

// x86_64 Legacy arch-specific boot package set
func x8664LegacyBootPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"dracut-config-generic",
			"grub2-pc",
		},
	}
}

// x86_64 UEFI arch-specific boot package set
func x8664UEFIBootPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"dracut-config-generic",
			"efibootmgr",
			"grub2-efi-x64",
			"shim-x64",
		},
	}
}

// aarch64 UEFI arch-specific boot package set
func aarch64UEFIBootPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"dracut-config-generic",
			"efibootmgr",
			"grub2-efi-aa64",
			"grub2-tools",
			"shim-aa64",
		},
	}
}

// ppc64le Legacy arch-specific boot package set
func ppc64leLegacyBootPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"dracut-config-generic",
			"grub2-ppc64le",
			"grub2-ppc64le-modules",
			"powerpc-utils",
		},
	}
}

// s390x Legacy arch-specific boot package set
func s390xLegacyBootPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{"dracut-config-generic", "s390utils-base"},
	}
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
