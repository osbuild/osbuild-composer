package rhel9

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
		Include: []string{
			"grub2-pc",
		},
	}
}

// ppc64le build package set
func ppc64leBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"grub2-ppc64le",
			"grub2-ppc64le-modules",
		},
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
				"grub2-efi-x64",
				"grub2-efi-x64-cdboot",
				"grub2-pc",
				"grub2-pc-modules",
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

func installerBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return distroBuildPackageSet(t).Append(
		rpmmd.PackageSet{
			Include: []string{
				"isomd5sum",
				"xorriso",
			},
		})
}

func anacondaBuildPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"squashfs-tools",
		},
	}

	ps = ps.Append(installerBuildPackageSet(t))
	ps = ps.Append(anacondaBootPackageSet(t))

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
			"powerpc-utils",
			"grub2-ppc64le",
			"grub2-ppc64le-modules",
		},
	}
}

// s390x Legacy arch-specific boot package set
func s390xLegacyBootPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"dracut-config-generic",
			"s390utils-base",
		},
	}
}

// OS package sets

// Replacement of the previously used @core package group
func coreOsCommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"audit",
			"basesystem",
			"bash",
			"coreutils",
			"cronie",
			"crypto-policies",
			"crypto-policies-scripts",
			"curl",
			"dnf",
			"yum",
			"e2fsprogs",
			"filesystem",
			"glibc",
			"grubby",
			"hostname",
			"iproute",
			"iproute-tc",
			"iputils",
			"kbd",
			"kexec-tools",
			"less",
			"logrotate",
			"man-db",
			"ncurses",
			"openssh-clients",
			"openssh-server",
			"p11-kit",
			"parted",
			"passwd",
			"policycoreutils",
			"procps-ng",
			"rootfiles",
			"rpm",
			"rpm-plugin-audit",
			"rsyslog",
			"selinux-policy-targeted",
			"setup",
			"shadow-utils",
			"sssd-common",
			"sssd-kcm",
			"sudo",
			"systemd",
			"tuned",
			"util-linux",
			"vim-minimal",
			"xfsprogs",
			"authselect",
			"prefixdevname",
			"dnf-plugins-core",
			"NetworkManager",
			"NetworkManager-team",
			"NetworkManager-tui",
			"libsysfs",
			"linux-firmware",
			"lshw",
			"lsscsi",
			"kernel-tools",
			"sg3_utils",
			"sg3_utils-libs",
			"python3-libselinux",
		},
	}

	// Do not include this in the distroSpecificPackageSet for now,
	// because it includes 'insights-client' which is not installed
	// by default on all RHEL images (although it would probably make sense).
	if t.arch.distro.isRHEL() {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"subscription-manager",
			},
		})
	}

	switch t.arch.Name() {
	case distro.X86_64ArchName:
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"irqbalance",
				"microcode_ctl",
			},
		})

	case distro.Aarch64ArchName:
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"irqbalance",
			},
		})

	case distro.Ppc64leArchName:
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"irqbalance",
				"opal-prd",
				"ppc64-diag-rtas",
				"powerpc-utils-core",
				"lsvpd",
			},
		})

	case distro.S390xArchName:
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"s390utils-core",
			},
		})
	}

	return ps
}

// packages that are only in some (sub)-distributions
func distroSpecificPackageSet(t *imageType) rpmmd.PackageSet {
	if t.arch.distro.isRHEL() {
		return rpmmd.PackageSet{
			Include: []string{
				"insights-client",
			},
		}
	}
	return rpmmd.PackageSet{}
}
