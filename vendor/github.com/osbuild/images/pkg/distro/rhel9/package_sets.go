package rhel9

// This file defines package sets that are used by more than one image type.

import (
	"fmt"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/rpmmd"
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

	case arch.ARCH_X86_64.String():
		ps = ps.Append(x8664BuildPackageSet(t))

	case arch.ARCH_PPC64LE.String():
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
	case arch.ARCH_X86_64.String():
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
	case arch.ARCH_AARCH64.String():
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
	case arch.ARCH_X86_64.String():
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"irqbalance",
				"microcode_ctl",
			},
		})

	case arch.ARCH_AARCH64.String():
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"irqbalance",
			},
		})

	case arch.ARCH_PPC64LE.String():
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"irqbalance",
				"opal-prd",
				"ppc64-diag-rtas",
				"powerpc-utils-core",
				"lsvpd",
			},
		})

	case arch.ARCH_S390X.String():
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
