package rhel85

// This file defines package sets that are used by more than one image type.

import "github.com/osbuild/osbuild-composer/internal/rpmmd"

// BUILD PACKAGE SETS

// distro-wide build package set
func distroBuildPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"dnf", "dosfstools", "e2fsprogs", "glibc", "lorax-templates-generic",
			"lorax-templates-rhel", "policycoreutils", "python36",
			"python3-iniparse", "qemu-img", "selinux-policy-targeted", "systemd",
			"tar", "xfsprogs", "xz",
		},
	}
}

// x86_64 build package set
func x8664BuildPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{"grub2-pc"},
	}
}

// ppc64le build package set
func ppc64leBuildPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{"grub2-ppc64le", "grub2-ppc64le-modules"},
	}
}

// common edge image build package set
func edgeBuildPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"efibootmgr", "genisoimage", "grub2-efi-ia32-cdboot",
			"grub2-efi-x64", "grub2-efi-x64-cdboot", "grub2-pc",
			"grub2-pc-modules", "grub2-tools", "grub2-tools-efi",
			"grub2-tools-extra", "grub2-tools-minimal", "isomd5sum",
			"rpm-ostree", "shim-ia32", "shim-x64", "squashfs-tools",
			"syslinux", "syslinux-nonlinux", "xorriso",
		},
		Exclude: nil,
	}
}

// x86_64 installer ISO build package set
// TODO: separate into common installer and arch specific sets
func x8664InstallerBuildPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"efibootmgr", "genisoimage", "grub2-efi-ia32-cdboot",
			"grub2-efi-x64", "grub2-efi-x64-cdboot", "grub2-pc",
			"grub2-pc-modules", "grub2-tools", "grub2-tools-efi",
			"grub2-tools-extra", "grub2-tools-minimal", "isomd5sum",
			"rpm-ostree", "shim-ia32", "shim-x64", "squashfs-tools",
			"syslinux", "syslinux-nonlinux", "xorriso",
		},
	}
}

// BOOT PACKAGE SETS

// x86_64 arch-specific boot package set
func x8664BootPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{"dracut-config-generic", "grub2-pc", "grub2-efi-x64", "shim-x64"},
	}
}

// aarch64 arch-specific boot package set
func aarch64BootPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"dracut-config-generic", "efibootmgr", "grub2-efi-aa64",
			"grub2-tools", "shim-aa64",
		},
	}
}

// ppc64le arch-specific boot package set
func ppc64leBootPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"dracut-config-generic", "powerpc-utils", "grub2-ppc64le",
			"grub2-ppc64le-modules",
		},
	}
}

// s390x arch-specific boot package set
func s390xBootPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{"dracut-config-generic", "s390utils-base"},
	}
}

// OS package sets

func x8664EdgeCommitPackageSet() rpmmd.PackageSet {
	x8664set := rpmmd.PackageSet{
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
	return edgeCommitCommonPackageSet().Append(x8664set)
}

func aarch64EdgeCommitPackageSet() rpmmd.PackageSet {
	aarch64set := rpmmd.PackageSet{
		Include: []string{"grub2-efi-aa64", "efibootmgr", "shim-aa64", "iwl7260-firmware"},
		Exclude: nil,
	}
	return edgeCommitCommonPackageSet().Append(aarch64set)
}

func qcow2CommonPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core", "authselect-compat", "chrony", "cloud-init",
			"cloud-utils-growpart", "cockpit-system", "cockpit-ws",
			"dhcp-client", "dnf", "dnf-utils", "dosfstools", "dracut-norescue",
			"insights-client", "NetworkManager", "net-tools", "nfs-utils",
			"oddjob", "oddjob-mkhomedir", "psmisc", "python3-jsonschema",
			"qemu-guest-agent", "redhat-release", "redhat-release-eula",
			"rsync", "subscription-manager-cockpit", "tar", "tcpdump", "yum",
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
	}
}

func vhdCommonPackageSet() rpmmd.PackageSet {
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
	}
}

func vmdkCommonPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core", "chrony", "firewalld", "langpacks-en", "open-vm-tools",
			"selinux-policy-targeted",
		},
		Exclude: []string{
			"dracut-config-rescue", "rng-tools",
		},
	}

}

func openstackCommonPackageSet() rpmmd.PackageSet {
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
	}

}

// edge commit OS package set
func edgeCommitCommonPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
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
			"greenboot", "greenboot-grub2", "greenboot-rpm-ostree-grub2",
			"greenboot-reboot", "greenboot-status",
		},
		Exclude: []string{"rng-tools"},
	}
}

// INSTALLER PACKAGE SET
func installerPackageSet() rpmmd.PackageSet {
	// TODO: simplify
	return rpmmd.PackageSet{
		Include: []string{
			"aajohan-comfortaa-fonts", "abattis-cantarell-fonts",
			"alsa-firmware", "alsa-tools-firmware", "anaconda",
			"anaconda-dracut", "anaconda-install-env-deps", "anaconda-widgets",
			"audit", "bind-utils", "biosdevname", "bitmap-fangsongti-fonts",
			"bzip2", "cryptsetup", "curl", "dbus-x11", "dejavu-sans-fonts",
			"dejavu-sans-mono-fonts", "device-mapper-persistent-data",
			"dmidecode", "dnf", "dracut-config-generic", "dracut-network",
			"dump", "efibootmgr", "ethtool", "ftp", "gdb-gdbserver", "gdisk",
			"gfs2-utils", "glibc-all-langpacks",
			"google-noto-sans-cjk-ttc-fonts", "grub2-efi-ia32-cdboot",
			"grub2-efi-x64-cdboot", "grub2-tools", "grub2-tools-efi",
			"grub2-tools-extra", "grub2-tools-minimal", "grubby",
			"gsettings-desktop-schemas", "hdparm", "hexedit", "hostname",
			"initscripts", "ipmitool", "iwl1000-firmware", "iwl100-firmware",
			"iwl105-firmware", "iwl135-firmware", "iwl2000-firmware",
			"iwl2030-firmware", "iwl3160-firmware", "iwl3945-firmware",
			"iwl4965-firmware", "iwl5000-firmware", "iwl5150-firmware",
			"iwl6000-firmware", "iwl6000g2a-firmware", "iwl6000g2b-firmware",
			"iwl6050-firmware", "iwl7260-firmware", "jomolhari-fonts",
			"kacst-farsi-fonts", "kacst-qurn-fonts", "kbd", "kbd-misc",
			"kdump-anaconda-addon", "kernel", "khmeros-base-fonts", "less",
			"libblockdev-lvm-dbus", "libertas-sd8686-firmware",
			"libertas-sd8787-firmware", "libertas-usb8388-firmware",
			"libertas-usb8388-olpc-firmware", "libibverbs",
			"libreport-plugin-bugzilla", "libreport-plugin-reportuploader",
			"libreport-rhel-anaconda-bugzilla", "librsvg2", "linux-firmware",
			"lklug-fonts", "lohit-assamese-fonts", "lohit-bengali-fonts",
			"lohit-devanagari-fonts", "lohit-gujarati-fonts",
			"lohit-gurmukhi-fonts", "lohit-kannada-fonts", "lohit-odia-fonts",
			"lohit-tamil-fonts", "lohit-telugu-fonts", "lsof", "madan-fonts",
			"memtest86+", "metacity", "mtr", "mt-st", "net-tools", "nfs-utils",
			"nmap-ncat", "nm-connection-editor", "nss-tools",
			"openssh-clients", "openssh-server", "oscap-anaconda-addon",
			"ostree", "pciutils", "perl-interpreter", "pigz", "plymouth",
			"prefixdevname", "python3-pyatspi", "rdma-core",
			"redhat-release-eula", "rng-tools", "rpcbind", "rpm-ostree",
			"rsync", "rsyslog", "selinux-policy-targeted", "sg3_utils",
			"shim-ia32", "shim-x64", "sil-abyssinica-fonts",
			"sil-padauk-fonts", "sil-scheherazade-fonts", "smartmontools",
			"smc-meera-fonts", "spice-vdagent", "strace", "syslinux",
			"systemd", "system-storage-manager", "tar",
			"thai-scalable-waree-fonts", "tigervnc-server-minimal",
			"tigervnc-server-module", "udisks2", "udisks2-iscsi", "usbutils",
			"vim-minimal", "volume_key", "wget", "xfsdump", "xfsprogs",
			"xorg-x11-drivers", "xorg-x11-fonts-misc", "xorg-x11-server-utils",
			"xorg-x11-server-Xorg", "xorg-x11-xauth", "xz",
		},
	}
}

func edgeInstallerPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"aajohan-comfortaa-fonts", "abattis-cantarell-fonts",
			"alsa-firmware", "alsa-tools-firmware", "anaconda",
			"anaconda-dracut", "anaconda-install-env-deps", "anaconda-widgets",
			"audit", "bind-utils", "biosdevname", "bitmap-fangsongti-fonts",
			"bzip2", "cryptsetup", "curl", "dbus-x11", "dejavu-sans-fonts",
			"dejavu-sans-mono-fonts", "device-mapper-persistent-data",
			"dmidecode", "dnf", "dracut-config-generic", "dracut-network",
			"dump", "efibootmgr", "ethtool", "ftp", "gdb-gdbserver", "gdisk",
			"gfs2-utils", "glibc-all-langpacks",
			"google-noto-sans-cjk-ttc-fonts", "grub2-efi-ia32-cdboot",
			"grub2-efi-x64-cdboot", "grub2-tools", "grub2-tools-efi",
			"grub2-tools-extra", "grub2-tools-minimal", "grubby",
			"gsettings-desktop-schemas", "hdparm", "hexedit", "hostname",
			"initscripts", "ipmitool", "iwl1000-firmware", "iwl100-firmware",
			"iwl105-firmware", "iwl135-firmware", "iwl2000-firmware",
			"iwl2030-firmware", "iwl3160-firmware", "iwl3945-firmware",
			"iwl4965-firmware", "iwl5000-firmware", "iwl5150-firmware",
			"iwl6000-firmware", "iwl6000g2a-firmware", "iwl6000g2b-firmware",
			"iwl6050-firmware", "iwl7260-firmware", "jomolhari-fonts",
			"kacst-farsi-fonts", "kacst-qurn-fonts", "kbd", "kbd-misc",
			"kdump-anaconda-addon", "kernel", "khmeros-base-fonts", "less",
			"libblockdev-lvm-dbus", "libertas-sd8686-firmware",
			"libertas-sd8787-firmware", "libertas-usb8388-firmware",
			"libertas-usb8388-olpc-firmware", "libibverbs",
			"libreport-plugin-bugzilla", "libreport-plugin-reportuploader",
			"libreport-rhel-anaconda-bugzilla", "librsvg2", "linux-firmware",
			"lklug-fonts", "lohit-assamese-fonts", "lohit-bengali-fonts",
			"lohit-devanagari-fonts", "lohit-gujarati-fonts",
			"lohit-gurmukhi-fonts", "lohit-kannada-fonts", "lohit-odia-fonts",
			"lohit-tamil-fonts", "lohit-telugu-fonts", "lsof", "madan-fonts",
			"memtest86+", "metacity", "mtr", "mt-st", "net-tools", "nfs-utils",
			"nmap-ncat", "nm-connection-editor", "nss-tools",
			"openssh-clients", "openssh-server", "oscap-anaconda-addon",
			"ostree", "pciutils", "perl-interpreter", "pigz", "plymouth",
			"prefixdevname", "python3-pyatspi", "rdma-core",
			"redhat-release-eula", "rng-tools", "rpcbind", "rpm-ostree",
			"rsync", "rsyslog", "selinux-policy-targeted", "sg3_utils",
			"shim-ia32", "shim-x64", "sil-abyssinica-fonts",
			"sil-padauk-fonts", "sil-scheherazade-fonts", "smartmontools",
			"smc-meera-fonts", "spice-vdagent", "strace", "syslinux",
			"systemd", "system-storage-manager", "tar",
			"thai-scalable-waree-fonts", "tigervnc-server-minimal",
			"tigervnc-server-module", "udisks2", "udisks2-iscsi", "usbutils",
			"vim-minimal", "volume_key", "wget", "xfsdump", "xfsprogs",
			"xorg-x11-drivers", "xorg-x11-fonts-misc", "xorg-x11-server-utils",
			"xorg-x11-server-Xorg", "xorg-x11-xauth", "xz",
		},
		Exclude: nil,
	}
}
