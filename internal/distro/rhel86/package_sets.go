package rhel86

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
			"dnf", "dosfstools", "e2fsprogs", "glibc", "lorax-templates-generic",
			"lorax-templates-rhel", "policycoreutils", "python36",
			"python3-iniparse", "qemu-img", "selinux-policy-targeted", "systemd",
			"tar", "xfsprogs", "xz",
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

// common ec2 image build package set
func ec2BuildPackageSet(t *imageType) rpmmd.PackageSet {
	return distroBuildPackageSet(t).Append(
		rpmmd.PackageSet{
			Include: []string{"python3-pyyaml"},
		})
}

// common edge image build package set
func edgeBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return distroBuildPackageSet(t).Append(
		rpmmd.PackageSet{
			Include: []string{"rpm-ostree"},
			Exclude: nil,
		})
}

func edgeRawImageBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return edgeBuildPackageSet(t).Append(
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
		Include: []string{"dracut-config-generic", "grub2-pc"},
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
			"dracut-config-generic", "efibootmgr", "grub2-efi-aa64",
			"grub2-tools", "shim-aa64",
		},
	}
}

// ppc64le Legacy arch-specific boot package set
func ppc64leLegacyBootPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"dracut-config-generic", "powerpc-utils", "grub2-ppc64le",
			"grub2-ppc64le-modules",
		},
	}
}

// s390x Legacy arch-specific boot package set
func s390xLegacyBootPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{"dracut-config-generic", "s390utils-base"},
	}
}

// OS package sets

func qcow2CommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"@core", "authselect-compat", "chrony", "cloud-init",
			"cloud-utils-growpart", "cockpit-system", "cockpit-ws",
			"dhcp-client", "dnf", "dnf-utils", "dosfstools", "dracut-norescue",
			"NetworkManager", "net-tools", "nfs-utils", "oddjob",
			"oddjob-mkhomedir", "psmisc", "python3-jsonschema",
			"qemu-guest-agent", "redhat-release", "redhat-release-eula",
			"rsync", "tar", "tcpdump", "yum",
		},
		Exclude: []string{
			"aic94xx-firmware", "alsa-firmware", "alsa-lib",
			"alsa-tools-firmware", "biosdevname", "dnf-plugin-spacewalk",
			"dracut-config-rescue", "fedora-release", "fedora-repos",
			"firewalld", "fwupd", "iprutils", "ivtv-firmware",
			"iwl100-firmware", "iwl1000-firmware", "iwl105-firmware",
			"iwl135-firmware", "iwl2000-firmware", "iwl2030-firmware",
			"iwl3160-firmware", "iwl3945-firmware", "iwl4965-firmware",
			"iwl5000-firmware", "iwl5150-firmware", "iwl6000-firmware",
			"iwl6000g2a-firmware", "iwl6000g2b-firmware", "iwl6050-firmware",
			"iwl7260-firmware", "langpacks-*", "langpacks-en", "langpacks-en",
			"libertas-sd8686-firmware", "libertas-sd8787-firmware",
			"libertas-usb8388-firmware", "nss", "plymouth", "rng-tools",
			"udisks2",
		},
	}.Append(bootPackageSet(t)).Append(distroSpecificPackageSet(t))

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
	return rpmmd.PackageSet{
		Include: []string{
			// Defaults
			"@Core", "langpacks-en",

			// From the lorax kickstart
			"selinux-policy-targeted", "chrony", "WALinuxAgent", "python3",
			"net-tools", "cloud-init", "cloud-utils-growpart", "gdisk",

			// removed from defaults but required to boot in azure
			"dhcp-client",
		},
		Exclude: []string{
			"dracut-config-rescue", "rng-tools",
		},
	}.Append(bootPackageSet(t))
}

func vmdkCommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core", "chrony", "firewalld", "langpacks-en", "open-vm-tools",
			"selinux-policy-targeted",
		},
		Exclude: []string{
			"dracut-config-rescue", "rng-tools",
		},
	}.Append(bootPackageSet(t))

}

func openstackCommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			// Defaults
			"@Core", "langpacks-en",

			// From the lorax kickstart
			"selinux-policy-targeted", "cloud-init", "qemu-guest-agent",
			"spice-vdagent",
		},
		Exclude: []string{
			"dracut-config-rescue", "rng-tools",
		},
	}.Append(bootPackageSet(t))

}

func ec2CommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core", "authselect-compat", "chrony", "cloud-init",
			"cloud-utils-growpart", "dhcp-client", "yum-utils",
			"dracut-config-generic", "dracut-norescue", "gdisk", "grub2",
			"langpacks-en", "NetworkManager", "NetworkManager-cloud-setup",
			"redhat-release", "redhat-release-eula", "rsync", "tar",
			"qemu-guest-agent",
		},
		Exclude: []string{
			"aic94xx-firmware", "alsa-firmware",
			"alsa-tools-firmware", "biosdevname", "firewalld", "iprutils", "ivtv-firmware",
			"iwl1000-firmware", "iwl100-firmware", "iwl105-firmware",
			"iwl135-firmware", "iwl2000-firmware", "iwl2030-firmware",
			"iwl3160-firmware", "iwl3945-firmware", "iwl4965-firmware",
			"iwl5000-firmware", "iwl5150-firmware", "iwl6000-firmware",
			"iwl6000g2a-firmware", "iwl6000g2b-firmware", "iwl6050-firmware",
			"iwl7260-firmware", "libertas-sd8686-firmware",
			"libertas-sd8787-firmware", "libertas-usb8388-firmware",
			"plymouth",
		},
	}.Append(bootPackageSet(t)).Append(distroSpecificPackageSet(t))
}

// rhel-ec2 image package set
func rhelEc2PackageSet(t *imageType) rpmmd.PackageSet {
	ec2PackageSet := ec2CommonPackageSet(t)
	ec2PackageSet.Include = append(ec2PackageSet.Include, "rh-amazon-rhui-client")
	ec2PackageSet.Exclude = append(ec2PackageSet.Exclude, "alsa-lib")
	return ec2PackageSet
}

// rhel-ha-ec2 image package set
func rhelEc2HaPackageSet(t *imageType) rpmmd.PackageSet {
	ec2HaPackageSet := ec2CommonPackageSet(t)
	ec2HaPackageSet.Include = append(ec2HaPackageSet.Include,
		"fence-agents-all",
		"pacemaker",
		"pcs",
		"rh-amazon-rhui-client-ha",
	)
	ec2HaPackageSet.Exclude = append(ec2HaPackageSet.Exclude, "alsa-lib")
	return ec2HaPackageSet
}

// rhel-sap-ec2 image package set
func rhelEc2SapPackageSet(t *imageType) rpmmd.PackageSet {
	ec2SapPackageSet := ec2CommonPackageSet(t)
	ec2SapPackageSet.Include = append(ec2SapPackageSet.Include,
		// SAP System Roles
		// https://access.redhat.com/sites/default/files/attachments/rhel_system_roles_for_sap_1.pdf
		"ansible",
		"rhel-system-roles-sap",
		// RHBZ#1959813
		"bind-utils",
		"compat-sap-c++-9",
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
		"libpng12",
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
	)
	return ec2SapPackageSet
}

// edge commit OS package set
func edgeCommitPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"redhat-release", "glibc", "glibc-minimal-langpack",
			"nss-altfiles", "dracut-config-generic", "dracut-network",
			"basesystem", "bash", "platform-python", "shadow-utils", "chrony",
			"setup", "shadow-utils", "sudo", "systemd", "coreutils",
			"util-linux", "curl", "vim-minimal", "rpm", "rpm-ostree", "polkit",
			"lvm2", "cryptsetup", "pinentry", "e2fsprogs", "dosfstools",
			"keyutils", "gnupg2", "attr", "xz", "gzip", "firewalld",
			"iptables", "NetworkManager", "NetworkManager-wifi",
			"NetworkManager-wwan", "wpa_supplicant", "dnsmasq", "traceroute",
			"hostname", "iproute", "iputils", "openssh-clients", "procps-ng",
			"rootfiles", "openssh-server", "passwd", "policycoreutils",
			"policycoreutils-python-utils", "selinux-policy-targeted",
			"setools-console", "less", "tar", "rsync", "fwupd", "usbguard",
			"bash-completion", "tmux", "ima-evm-utils", "audit", "podman",
			"container-selinux", "skopeo", "criu", "slirp4netns",
			"fuse-overlayfs", "clevis", "clevis-dracut", "clevis-luks",
			"greenboot", "greenboot-default-health-checks",
			"fdo-client", "fdo-owner-cli",
		},
		Exclude: []string{"rng-tools"},
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
			"grub2", "grub2-efi-x64", "efibootmgr", "shim-x64",
			"microcode_ctl", "iwl1000-firmware", "iwl100-firmware",
			"iwl105-firmware", "iwl135-firmware", "iwl2000-firmware",
			"iwl2030-firmware", "iwl3160-firmware", "iwl5000-firmware",
			"iwl5150-firmware", "iwl6000-firmware", "iwl6050-firmware",
			"iwl7260-firmware",
		},
		Exclude: nil,
	}
}

func aarch64EdgeCommitPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{"grub2-efi-aa64", "efibootmgr", "shim-aa64", "iwl7260-firmware"},
		Exclude: nil,
	}
}

func bareMetalPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"authselect-compat", "chrony", "cockpit-system", "cockpit-ws",
			"@core", "dhcp-client", "dnf", "dnf-utils", "dosfstools",
			"dracut-norescue", "iwl1000-firmware",
			"iwl100-firmware", "iwl105-firmware", "iwl135-firmware",
			"iwl2000-firmware", "iwl2030-firmware", "iwl3160-firmware",
			"iwl3945-firmware", "iwl4965-firmware", "iwl5000-firmware",
			"iwl5150-firmware", "iwl6000-firmware", "iwl6000g2a-firmware",
			"iwl6000g2b-firmware", "iwl6050-firmware", "iwl7260-firmware",
			"lvm2", "net-tools", "NetworkManager", "nfs-utils", "oddjob",
			"oddjob-mkhomedir", "policycoreutils", "psmisc",
			"python3-jsonschema", "qemu-guest-agent", "redhat-release",
			"redhat-release-eula", "rsync", "selinux-policy-targeted",
			"tar", "tcpdump", "yum",
		},
		Exclude: nil,
	}.Append(bootPackageSet(t)).Append(distroSpecificPackageSet(t))

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
			Include: []string{"insights-client"},
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
			"iwl6000-firmware",
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
			"anaconda-install-env-deps",
			"anaconda-widgets",
			"audit",
			"bind-utils",
			"bitmap-fangsongti-fonts",
			"bzip2",
			"cryptsetup",
			"dbus-x11",
			"dejavu-sans-fonts",
			"dejavu-sans-mono-fonts",
			"device-mapper-persistent-data",
			"dnf",
			"dump",
			"ethtool",
			"fcoe-utils",
			"ftp",
			"gdb-gdbserver",
			"gdisk",
			"gfs2-utils",
			"glibc-all-langpacks",
			"google-noto-sans-cjk-ttc-fonts",
			"gsettings-desktop-schemas",
			"hdparm",
			"hexedit",
			"gsettings-desktop-schemas",
			"hdparm",
			"hexedit",
			"initscripts",
			"ipmitool",
			"iwl3945-firmware",
			"iwl4965-firmware",
			"iwl6000g2a-firmware",
			"iwl6000g2b-firmware",
			"jomolhari-fonts",
			"kacst-farsi-fonts",
			"kacst-qurn-fonts",
			"kbd",
			"kbd-misc",
			"kdump-anaconda-addon",
			"khmeros-base-fonts",
			"libblockdev-lvm-dbus",
			"libertas-sd8686-firmware",
			"libertas-sd8787-firmware",
			"libertas-usb8388-firmware",
			"libertas-usb8388-olpc-firmware",
			"libibverbs",
			"libreport-plugin-bugzilla",
			"libreport-plugin-reportuploader",
			"libreport-rhel-anaconda-bugzilla",
			"librsvg2",
			"linux-firmware",
			"lklug-fonts",
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
			"metacity",
			"mt-st",
			"mtr",
			"net-tools",
			"nm-connection-editor",
			"nmap-ncat",
			"nss-tools",
			"openssh-server",
			"oscap-anaconda-addon",
			"pciutils",
			"perl-interpreter",
			"pigz",
			"python3-pyatspi",
			"rdma-core",
			"redhat-release-eula",
			"rpm-ostree",
			"rsync",
			"rsyslog",
			"sg3_utils",
			"sil-abyssinica-fonts",
			"sil-padauk-fonts",
			"sil-scheherazade-fonts",
			"smartmontools",
			"smc-meera-fonts",
			"spice-vdagent",
			"strace",
			"system-storage-manager",
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
			"xorg-x11-drivers",
			"xorg-x11-fonts-misc",
			"xorg-x11-server-Xorg",
			"xorg-x11-server-utils",
			"xorg-x11-xauth",
		},
	})

	ps = ps.Append(anacondaBootPackageSet(t))

	switch t.arch.Name() {
	case distro.X86_64ArchName:
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"biosdevname",
				"dmidecode",
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
			"cloud-utils-growpart",
			"coreos-installer",
			"coreos-installer-dracut",
			"fdo-init",
			"coreutils",
			"device-mapper-multipath",
			"dnsmasq",
			"dosfstools",
			"dracut-live",
			"e2fsprogs",
			"fcoe-utils",
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
		Exclude: nil,
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
