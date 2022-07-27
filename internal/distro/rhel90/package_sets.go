package rhel90

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

// common ec2 image build package set
func ec2BuildPackageSet(t *imageType) rpmmd.PackageSet {
	return distroBuildPackageSet(t).Append(
		rpmmd.PackageSet{
			Include: []string{
				"python3-pyyaml",
			},
		})
}

// common edge image build package set
func edgeBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return distroBuildPackageSet(t).Append(
		rpmmd.PackageSet{
			Include: []string{
				"rpm-ostree",
			},
		})
}

func edgeEncryptionBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"clevis",
			"clevis-luks",
			"cryptsetup",
		},
	}
}

func edgeSimplifiedInstallerBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return edgeInstallerBuildPackageSet(t).Append(
		edgeEncryptionBuildPackageSet(t),
	)
}

func edgeRawImageBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return edgeBuildPackageSet(t).Append(edgeEncryptionBuildPackageSet(t)).Append(
		bootPackageSet(t),
	)
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

func edgeInstallerBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return anacondaBuildPackageSet(t).Append(
		edgeBuildPackageSet(t),
	)
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

func qcow2CommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"authselect-compat",
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"cockpit-system",
			"cockpit-ws",
			"dnf-utils",
			"dosfstools",
			"nfs-utils",
			"oddjob",
			"oddjob-mkhomedir",
			"psmisc",
			"python3-jsonschema",
			"qemu-guest-agent",
			"redhat-release",
			"redhat-release-eula",
			"rsync",
			"tar",
			"tcpdump",
		},
		Exclude: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-lib",
			"alsa-tools-firmware",
			"biosdevname",
			"dnf-plugin-spacewalk",
			"fedora-release",
			"fedora-repos",
			"iprutils",
			"ivtv-firmware",
			"langpacks-*",
			"langpacks-en",
			"libertas-sd8787-firmware",
			"nss",
			"plymouth",
			"rng-tools",
			"udisks2",
		},
	}.Append(bootPackageSet(t)).Append(coreOsCommonPackageSet(t)).Append(distroSpecificPackageSet(t))

	// Ensure to not pull in subscription-manager on non-RHEL distro
	if t.arch.distro.isRHEL() {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"subscription-manager-cockpit",
			},
		})
	}

	return ps
}

func vhdCommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			// Defaults
			"langpacks-en",
			// From the lorax kickstart
			"chrony",
			"firewalld",
			"WALinuxAgent",
			"python3",
			"net-tools",
			"cloud-init",
			"cloud-utils-growpart",
			"gdisk",

			// removed from defaults but required to boot in azure
			"dhcp-client",
		},
		Exclude: []string{
			"rng-tools",
		},
	}.Append(bootPackageSet(t)).Append(coreOsCommonPackageSet(t))

	if t.arch.Name() == distro.X86_64ArchName {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				// packages below used to come from @core group and were not excluded
				// they may not be needed at all, but kept them here to not need
				// to exclude them instead in all other images
				"iwl100-firmware",
				"iwl105-firmware",
				"iwl135-firmware",
				"iwl1000-firmware",
				"iwl2000-firmware",
				"iwl2030-firmware",
				"iwl3160-firmware",
				"iwl5000-firmware",
				"iwl5150-firmware",
				"iwl6000g2a-firmware",
				"iwl6050-firmware",
				"iwl7260-firmware",
			},
		})
	}

	return ps
}

func azureRhuiCommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@Server",
			"bzip2",
			"cloud-init",
			"cloud-utils-growpart",
			"dracut-config-generic",
			"efibootmgr",
			"gdisk",
			"grub2-efi-x64",
			"grub2-pc",
			"hyperv-daemons",
			"insights-client",
			"kernel-core",
			"kernel-modules",
			"kernel",
			"langpacks-en",
			"lvm2",
			"NetworkManager",
			"NetworkManager-cloud-setup",
			"nvme-cli",
			"patch",
			"rhc",
			"rhui-azure-rhel9",
			"rng-tools",
			"selinux-policy-targeted",
			"shim-x64",
			"uuid",
			"WALinuxAgent",
			"yum-utils",
		},
		Exclude: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-lib",
			"alsa-sof-firmware",
			"alsa-tools-firmware",
			"biosdevname",
			"bolt",
			"buildah",
			"cockpit-podman",
			"containernetworking-plugins",
			"dnf-plugin-spacewalk",
			"dracut-config-rescue",
			"glibc-all-langpacks",
			"iprutils",
			"ivtv-firmware",
			"iwl100-firmware",
			"iwl1000-firmware",
			"iwl105-firmware",
			"iwl135-firmware",
			"iwl2000-firmware",
			"iwl2030-firmware",
			"iwl3160-firmware",
			"iwl3945-firmware",
			"iwl4965-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6000-firmware",
			"iwl6000g2a-firmware",
			"iwl6000g2b-firmware",
			"iwl6050-firmware",
			"iwl7260-firmware",
			"libertas-sd8686-firmware",
			"libertas-sd8787-firmware",
			"libertas-usb8388-firmware",
			"NetworkManager-config-server",
			"plymouth",
			"podman",
			"python3-dnf-plugin-spacewalk",
			"python3-hwdata",
			"python3-rhnlib",
			"rhn-check",
			"rhn-client-tools",
			"rhn-setup",
			"rhnlib",
			"rhnsd",
			"usb_modeswitch",
		},
	}.Append(bootPackageSet(t))
}

func vmdkCommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"chrony",
			"cloud-init",
			"firewalld",
			"langpacks-en",
			"open-vm-tools",
		},
		Exclude: []string{
			"rng-tools",
		},
	}.Append(bootPackageSet(t)).Append(coreOsCommonPackageSet(t))

	if t.arch.Name() == distro.X86_64ArchName {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				// packages below used to come from @core group and were not excluded
				// they may not be needed at all, but kept them here to not need
				// to exclude them instead in all other images
				"iwl100-firmware",
				"iwl105-firmware",
				"iwl135-firmware",
				"iwl1000-firmware",
				"iwl2000-firmware",
				"iwl2030-firmware",
				"iwl3160-firmware",
				"iwl5000-firmware",
				"iwl5150-firmware",
				"iwl6000g2a-firmware",
				"iwl6050-firmware",
				"iwl7260-firmware",
			},
		})
	}

	return ps
}

func openstackCommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			// Defaults
			"langpacks-en",
			"firewalld",

			// From the lorax kickstart
			"cloud-init",
			"qemu-guest-agent",
			"spice-vdagent",
		},
		Exclude: []string{
			"rng-tools",
		},
	}.Append(bootPackageSet(t)).Append(coreOsCommonPackageSet(t))

	if t.arch.Name() == distro.X86_64ArchName {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				// packages below used to come from @core group and were not excluded
				// they may not be needed at all, but kept them here to not need
				// to exclude them instead in all other images
				"iwl100-firmware",
				"iwl105-firmware",
				"iwl135-firmware",
				"iwl1000-firmware",
				"iwl2000-firmware",
				"iwl2030-firmware",
				"iwl3160-firmware",
				"iwl5000-firmware",
				"iwl5150-firmware",
				"iwl6000g2a-firmware",
				"iwl6050-firmware",
				"iwl7260-firmware",
			},
		})
	}

	return ps
}

func ec2CommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"authselect-compat",
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"dhcp-client",
			"yum-utils",
			"dracut-config-generic",
			"gdisk",
			"grub2",
			"langpacks-en",
			"NetworkManager-cloud-setup",
			"redhat-release",
			"redhat-release-eula",
			"rsync",
			"tar",
		},
		Exclude: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-tools-firmware",
			"biosdevname",
			"iprutils",
			"ivtv-firmware",
			"libertas-sd8787-firmware",
			"plymouth",
			// RHBZ#2064087
			"dracut-config-rescue",
			// RHBZ#2075815
			"qemu-guest-agent",
		},
	}.Append(bootPackageSet(t)).Append(coreOsCommonPackageSet(t)).Append(distroSpecificPackageSet(t))
}

// rhel-ec2 image package set
func rhelEc2PackageSet(t *imageType) rpmmd.PackageSet {
	ec2PackageSet := ec2CommonPackageSet(t)
	ec2PackageSet = ec2PackageSet.Append(rpmmd.PackageSet{
		Include: []string{
			"rh-amazon-rhui-client",
		},
		Exclude: []string{
			"alsa-lib",
		},
	})
	return ec2PackageSet
}

// rhel-ha-ec2 image package set
func rhelEc2HaPackageSet(t *imageType) rpmmd.PackageSet {
	ec2HaPackageSet := ec2CommonPackageSet(t)
	ec2HaPackageSet = ec2HaPackageSet.Append(rpmmd.PackageSet{
		Include: []string{
			"fence-agents-all",
			"pacemaker",
			"pcs",
			"rh-amazon-rhui-client-ha",
		},
		Exclude: []string{
			"alsa-lib",
		},
	})
	return ec2HaPackageSet
}

// rhel-sap-ec2 image package set
func rhelEc2SapPackageSet(t *imageType) rpmmd.PackageSet {
	ec2SapPackageSet := ec2CommonPackageSet(t)
	ec2SapPackageSet = ec2SapPackageSet.Append(rpmmd.PackageSet{
		Include: []string{
			// RHBZ#2076763
			"@Server",
			// SAP System Roles
			// https://access.redhat.com/sites/default/files/attachments/rhel_system_roles_for_sap_1.pdf
			"ansible-core",
			"rhel-system-roles-sap",
			// RHBZ#1959813
			"bind-utils",
			"nfs-utils",
			"tcsh",
			// RHBZ#1959955
			"uuidd",
			// RHBZ#1959923
			"cairo",
			"expect",
			"graphviz",
			"gtk2",
			"iptraf-ng",
			"krb5-workstation",
			"libaio",
			"libatomic",
			"libcanberra-gtk2",
			"libicu",
			"libtool-ltdl",
			"lm_sensors",
			"net-tools",
			"numactl",
			"PackageKit-gtk3-module",
			"xorg-x11-xauth",
			// RHBZ#1960617
			"tuned-profiles-sap-hana",
			// RHBZ#1961168
			"libnsl",
			// RHUI client
			"rh-amazon-rhui-client-sap-bundle-e4s",
		},
	})
	return ec2SapPackageSet
}

// common GCE image
func gceCommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"langpacks-en", // not in Google's KS
			"acpid",
			"dhcp-client",
			"dnf-automatic",
			"net-tools",
			//"openssh-server", included in core
			"python3",
			"rng-tools",
			"tar",
			"vim",

			// GCE guest tools
			"google-compute-engine",
			"google-osconfig-agent",
			"gce-disk-expand",
			// GCP SDK
			"google-cloud-sdk",

			// Not explicitly included in GCP kickstart, but present on the image
			// for time synchronization
			"chrony",
			"timedatex",
			// EFI
			"grub2-tools-efi",
			"firewalld", // not pulled in any more as on RHEL-8
		},
		Exclude: []string{
			"alsa-utils",
			"b43-fwcutter",
			"dmraid",
			"eject",
			"gpm",
			"irqbalance",
			"microcode_ctl",
			"smartmontools",
			"aic94xx-firmware",
			"atmel-firmware",
			"b43-openfwwf",
			"bfa-firmware",
			"ipw2100-firmware",
			"ipw2200-firmware",
			"ivtv-firmware",
			"iwl100-firmware",
			"iwl1000-firmware",
			"iwl3945-firmware",
			"iwl4965-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6000-firmware",
			"iwl6000g2a-firmware",
			"iwl6050-firmware",
			"kernel-firmware",
			"libertas-usb8388-firmware",
			"ql2100-firmware",
			"ql2200-firmware",
			"ql23xx-firmware",
			"ql2400-firmware",
			"ql2500-firmware",
			"rt61pci-firmware",
			"rt73usb-firmware",
			"xorg-x11-drv-ati-firmware",
			"zd1211-firmware",
			// RHBZ#2075815
			"qemu-guest-agent",
		},
	}.Append(bootPackageSet(t)).Append(coreOsCommonPackageSet(t)).Append(distroSpecificPackageSet(t))

	// Some excluded packages are part of the @core group package set returned
	// by coreOsCommonPackageSet(). Ensure that the conflicting packages are
	// returned from the list of `Include` packages.
	return ps.ResolveConflictsExclude()
}

// GCE BYOS image
func gcePackageSet(t *imageType) rpmmd.PackageSet {
	return gceCommonPackageSet(t)
}

// GCE RHUI image
func gceRhuiPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"google-rhui-client-rhel9",
		},
	}.Append(gceCommonPackageSet(t))
}

// edge commit OS package set
func edgeCommitPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"redhat-release",
			"glibc",
			"glibc-minimal-langpack",
			"nss-altfiles",
			"dracut-config-generic",
			"dracut-network",
			"basesystem",
			"bash",
			"platform-python",
			"shadow-utils",
			"chrony",
			"setup",
			"shadow-utils",
			"sudo",
			"systemd",
			"coreutils",
			"util-linux",
			"curl",
			"vim-minimal",
			"rpm",
			"rpm-ostree",
			"polkit",
			"lvm2",
			"cryptsetup",
			"pinentry",
			"e2fsprogs",
			"dosfstools",
			"keyutils",
			"gnupg2",
			"attr",
			"xz",
			"gzip",
			"firewalld",
			"iptables",
			"NetworkManager",
			"NetworkManager-wifi",
			"NetworkManager-wwan",
			"wpa_supplicant",
			"dnsmasq",
			"traceroute",
			"hostname",
			"iproute",
			"iputils",
			"openssh-clients",
			"procps-ng",
			"rootfiles",
			"openssh-server",
			"passwd",
			"policycoreutils",
			"policycoreutils-python-utils",
			"selinux-policy-targeted",
			"setools-console",
			"less",
			"tar",
			"rsync",
			"usbguard",
			"bash-completion",
			"tmux",
			"ima-evm-utils",
			"audit",
			"podman",
			"container-selinux",
			"skopeo",
			"criu",
			"slirp4netns",
			"fuse-overlayfs",
			"clevis",
			"clevis-dracut",
			"clevis-luks",
			"greenboot",
			"greenboot-default-health-checks",
			"fdo-client",
			"fdo-owner-cli",
		},
		Exclude: []string{
			"rng-tools",
		},
	}

	ps = ps.Append(bootPackageSet(t))

	switch t.arch.Name() {
	case distro.X86_64ArchName:
		ps = ps.Append(x8664EdgeCommitPackageSet(t))

	case distro.Aarch64ArchName:
		ps = ps.Append(aarch64EdgeCommitPackageSet(t))
	}

	return ps

}

func x8664EdgeCommitPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"grub2",
			"grub2-efi-x64",
			"efibootmgr",
			"shim-x64",
			"microcode_ctl",
			"iwl1000-firmware",
			"iwl100-firmware",
			"iwl105-firmware",
			"iwl135-firmware",
			"iwl2000-firmware",
			"iwl2030-firmware",
			"iwl3160-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6050-firmware",
			"iwl7260-firmware",
		},
	}
}

func aarch64EdgeCommitPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"grub2-efi-aa64",
			"efibootmgr",
			"shim-aa64",
			"iwl7260-firmware",
		},
	}
}

func bareMetalPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"authselect-compat",
			"chrony",
			"cockpit-system",
			"cockpit-ws",
			"dhcp-client",
			"dnf-utils",
			"dosfstools",
			"firewalld",
			"iwl1000-firmware",
			"iwl100-firmware",
			"iwl105-firmware",
			"iwl135-firmware",
			"iwl2000-firmware",
			"iwl2030-firmware",
			"iwl3160-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6000g2a-firmware",
			"iwl6000g2b-firmware",
			"iwl6050-firmware",
			"iwl7260-firmware",
			"lvm2",
			"net-tools",
			"nfs-utils",
			"oddjob",
			"oddjob-mkhomedir",
			"policycoreutils",
			"psmisc",
			"python3-jsonschema",
			"qemu-guest-agent",
			"redhat-release",
			"redhat-release-eula",
			"rsync",
			"tar",
			"tcpdump",
		},
	}.Append(bootPackageSet(t)).Append(coreOsCommonPackageSet(t)).Append(distroBuildPackageSet(t))

	// Ensure to not pull in subscription-manager on non-RHEL distro
	if t.arch.distro.isRHEL() {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"subscription-manager-cockpit",
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

// INSTALLER PACKAGE SET

func installerPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"anaconda-dracut",
			"curl",
			"dracut-config-generic",
			"dracut-network",
			"hostname",
			"iwl100-firmware",
			"iwl1000-firmware",
			"iwl105-firmware",
			"iwl135-firmware",
			"iwl2000-firmware",
			"iwl2030-firmware",
			"iwl3160-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6050-firmware",
			"iwl7260-firmware",
			"kernel",
			"less",
			"nfs-utils",
			"openssh-clients",
			"ostree",
			"plymouth",
			"prefixdevname",
			"rng-tools",
			"rpcbind",
			"selinux-policy-targeted",
			"systemd",
			"tar",
			"xfsprogs",
			"xz",
		},
	}

	switch t.arch.Name() {
	case distro.X86_64ArchName:
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"biosdevname",
			},
		})
	}

	return ps
}

func anacondaPackageSet(t *imageType) rpmmd.PackageSet {

	// common installer packages
	ps := installerPackageSet(t)

	ps = ps.Append(rpmmd.PackageSet{
		Include: []string{
			"aajohan-comfortaa-fonts",
			"abattis-cantarell-fonts",
			"alsa-firmware",
			"alsa-tools-firmware",
			"anaconda",
			"anaconda-dracut",
			"anaconda-install-env-deps",
			"anaconda-widgets",
			"audit",
			"bind-utils",
			"bitmap-fangsongti-fonts",
			"bzip2",
			"cryptsetup",
			"curl",
			"dbus-x11",
			"dejavu-sans-fonts",
			"dejavu-sans-mono-fonts",
			"device-mapper-persistent-data",
			"dmidecode",
			"dnf",
			"dracut-config-generic",
			"dracut-network",
			"efibootmgr",
			"ethtool",
			"fcoe-utils",
			"ftp",
			"gdb-gdbserver",
			"gdisk",
			"glibc-all-langpacks",
			"gnome-kiosk",
			"google-noto-sans-cjk-ttc-fonts",
			"grub2-tools",
			"grub2-tools-extra",
			"grub2-tools-minimal",
			"grubby",
			"gsettings-desktop-schemas",
			"hdparm",
			"hexedit",
			"hostname",
			"initscripts",
			"ipmitool",
			"iwl1000-firmware",
			"iwl100-firmware",
			"iwl105-firmware",
			"iwl135-firmware",
			"iwl2000-firmware",
			"iwl2030-firmware",
			"iwl3160-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6000g2a-firmware",
			"iwl6000g2b-firmware",
			"iwl6050-firmware",
			"iwl7260-firmware",
			"jomolhari-fonts",
			"kacst-farsi-fonts",
			"kacst-qurn-fonts",
			"kbd",
			"kbd-misc",
			"kdump-anaconda-addon",
			"kernel",
			"khmeros-base-fonts",
			"less",
			"libblockdev-lvm-dbus",
			"libibverbs",
			"libreport-plugin-bugzilla",
			"libreport-plugin-reportuploader",
			"librsvg2",
			"linux-firmware",
			"lklug-fonts",
			"lldpad",
			"lohit-assamese-fonts",
			"lohit-bengali-fonts",
			"lohit-devanagari-fonts",
			"lohit-gujarati-fonts",
			"lohit-gurmukhi-fonts",
			"lohit-kannada-fonts",
			"lohit-odia-fonts",
			"lohit-tamil-fonts",
			"lohit-telugu-fonts",
			"lsof",
			"madan-fonts",
			"mtr",
			"mt-st",
			"net-tools",
			"nfs-utils",
			"nmap-ncat",
			"nm-connection-editor",
			"nss-tools",
			"nvme-cli", // for nvmf dracut module
			"openssh-clients",
			"openssh-server",
			"oscap-anaconda-addon",
			"ostree",
			"pciutils",
			"perl-interpreter",
			"pigz",
			"plymouth",
			"prefixdevname",
			"python3-pyatspi",
			"rdma-core",
			"redhat-release-eula",
			"rng-tools",
			"rpcbind",
			"rpm-ostree",
			"rsync",
			"rsyslog",
			"selinux-policy-targeted",
			"sg3_utils",
			"sil-abyssinica-fonts",
			"sil-padauk-fonts",
			"sil-scheherazade-fonts",
			"smartmontools",
			"smc-meera-fonts",
			"spice-vdagent",
			"strace",
			"systemd",
			"tar",
			"thai-scalable-waree-fonts",
			"tigervnc-server-minimal",
			"tigervnc-server-module",
			"udisks2",
			"udisks2-iscsi",
			"usbutils",
			"vim-minimal",
			"volume_key",
			"wget",
			"xfsdump",
			"xfsprogs",
			"xorg-x11-drivers",
			"xorg-x11-fonts-misc",
			"xorg-x11-server-utils",
			"xorg-x11-server-Xorg",
			"xorg-x11-xauth",
			"xz",
		},
	})

	ps = ps.Append(anacondaBootPackageSet(t))

	switch t.arch.Name() {
	case distro.X86_64ArchName:
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"biosdevname",
				"dmidecode",
				"grub2-tools-efi",
				"memtest86+",
			},
		})

	case distro.Aarch64ArchName:
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"dmidecode",
			},
		})

	default:
		panic(fmt.Sprintf("unsupported arch: %s", t.arch.Name()))
	}

	return ps
}

func edgeInstallerPackageSet(t *imageType) rpmmd.PackageSet {
	return anacondaPackageSet(t)
}

func edgeSimplifiedInstallerPackageSet(t *imageType) rpmmd.PackageSet {
	// common installer packages
	ps := installerPackageSet(t)

	ps = ps.Append(rpmmd.PackageSet{
		Include: []string{
			"attr",
			"basesystem",
			"binutils",
			"bsdtar",
			"clevis-dracut",
			"clevis-luks",
			"cloud-utils-growpart",
			"coreos-installer",
			"coreos-installer-dracut",
			"coreutils",
			"device-mapper-multipath",
			"dnsmasq",
			"dosfstools",
			"dracut-live",
			"e2fsprogs",
			"fcoe-utils",
			"fdo-init",
			"gzip",
			"ima-evm-utils",
			"iproute",
			"iptables",
			"iputils",
			"iscsi-initiator-utils",
			"keyutils",
			"lldpad",
			"lvm2",
			"passwd",
			"policycoreutils",
			"policycoreutils-python-utils",
			"procps-ng",
			"rootfiles",
			"setools-console",
			"sudo",
			"traceroute",
			"util-linux",
		},
	})

	switch t.arch.Name() {

	case distro.X86_64ArchName:
		ps = ps.Append(x8664EdgeCommitPackageSet(t))
	case distro.Aarch64ArchName:
		ps = ps.Append(aarch64EdgeCommitPackageSet(t))

	default:
		panic(fmt.Sprintf("unsupported arch: %s", t.arch.Name()))
	}

	return ps
}
