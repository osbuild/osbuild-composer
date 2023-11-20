package fedora

// This file defines package sets that are used by more than one image type.

import (
	"fmt"
	"strconv"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

func cloudBaseSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@Fedora Cloud Server",
			"chrony", // not mentioned in the kickstart, anaconda pulls it when setting the timezone
			"langpacks-en",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"firewalld",
			"geolite2-city",
			"geolite2-country",
			"plymouth",
		},
	}
}

func qcow2CommonPackageSet(t *imageType) rpmmd.PackageSet {
	return cloudBaseSet(t).Append(
		rpmmd.PackageSet{
			Include: []string{
				"qemu-guest-agent",
			},
		})
}

func vhdCommonPackageSet(t *imageType) rpmmd.PackageSet {
	return cloudBaseSet(t).Append(
		rpmmd.PackageSet{
			Include: []string{
				"WALinuxAgent",
			},
		})
}

func vmdkCommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@Fedora Cloud Server",
			"chrony",
			"systemd-udev",
			"langpacks-en",
			"open-vm-tools",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"etables",
			"firewalld",
			"geolite2-city",
			"geolite2-country",
			"gobject-introspection",
			"plymouth",
			"zram-generator-defaults",
			"grubby-deprecated",
			"extlinux-bootloader",
		},
	}
}

// fedora iot commit OS package set
func iotCommitPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"aardvark-dns",
			"atheros-firmware",
			"attr",
			"authselect",
			"basesystem",
			"bash",
			"bash-completion",
			"brcmfmac-firmware",
			"chrony",
			"clevis",
			"clevis-dracut",
			"clevis-luks",
			"clevis-pin-tpm2",
			"containernetworking-plugins",
			"container-selinux",
			"coreutils",
			"cracklib-dicts",
			"criu",
			"cryptsetup",
			"curl",
			"dbus-parsec",
			"dnsmasq",
			"dosfstools",
			"dracut-config-generic",
			"dracut-network",
			"e2fsprogs",
			"efibootmgr",
			"fedora-iot-config",
			"fedora-release-iot",
			"firewalld",
			"fwupd",
			"fwupd-efi",
			"fwupd-plugin-modem-manager",
			"fwupd-plugin-uefi-capsule-data",
			"glibc",
			"glibc-minimal-langpack",
			"gnupg2",
			"greenboot",
			"greenboot-default-health-checks",
			"gzip",
			"hostname",
			"ignition",
			"ima-evm-utils",
			"iproute",
			"iputils",
			"iwd",
			"iwlwifi-mvm-firmware",
			"kernel-tools",
			"keyutils",
			"less",
			"libsss_sudo",
			"linux-firmware",
			"lvm2",
			"netavark",
			"NetworkManager",
			"NetworkManager-wifi",
			"NetworkManager-wwan",
			"nss-altfiles",
			"openssh-clients",
			"openssh-server",
			"openssl",
			"parsec",
			"passwd",
			"pinentry",
			"podman",
			"podman-plugins",
			"policycoreutils",
			"policycoreutils-python-utils",
			"polkit",
			"procps-ng",
			"realtek-firmware",
			"rootfiles",
			"rpm",
			"screen",
			"selinux-policy-targeted",
			"setools-console",
			"setup",
			"shadow-utils",
			"skopeo",
			"slirp4netns",
			"sssd-client",
			"sudo",
			"systemd",
			"systemd-resolved",
			"tar",
			"tmux",
			"tpm2-pkcs11",
			"traceroute",
			"usbguard",
			"util-linux",
			"vim-minimal",
			"wireless-regdb",
			"wpa_supplicant",
			"xfsprogs",
			"xz",
			"zezere-ignition",
			"zram-generator",
		},
	}

	if !common.VersionLessThan(t.arch.distro.osVersion, "38") {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"fdo-client",
				"fdo-owner-cli",
				"ignition-edge",
				"ssh-key-dir",
			},
		})
	}

	return ps
}

// INSTALLER PACKAGE SET

func installerPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"anaconda-dracut",
			"atheros-firmware",
			"brcmfmac-firmware",
			"curl",
			"dracut-config-generic",
			"dracut-network",
			"hostname",
			"iwlwifi-dvm-firmware",
			"iwlwifi-mvm-firmware",
			"kernel",
			"linux-firmware",
			"less",
			"nfs-utils",
			"openssh-clients",
			"ostree",
			"plymouth",
			"realtek-firmware",
			"rng-tools",
			"rpcbind",
			"selinux-policy-targeted",
			"systemd",
			"tar",
			"xfsprogs",
			"xz",
		},
	}
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
			"atheros-firmware",
			"audit",
			"bind-utils",
			"bitmap-fangsongti-fonts",
			"brcmfmac-firmware",
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
			"iwlwifi-dvm-firmware",
			"iwlwifi-mvm-firmware",
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
			"openssh-clients",
			"openssh-server",
			"oscap-anaconda-addon",
			"ostree",
			"pciutils",
			"perl-interpreter",
			"pigz",
			"plymouth",
			"python3-pyatspi",
			"rdma-core",
			"realtek-firmware",
			"rit-meera-new-fonts",
			"rng-tools",
			"rpcbind",
			"rpm-ostree",
			"rsync",
			"rsyslog",
			"selinux-policy-targeted",
			"sg3_utils",
			"sil-abyssinica-fonts",
			"sil-padauk-fonts",
			"sil-scheherazade-new-fonts",
			"smartmontools",
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
			"xorg-x11-server-Xorg",
			"xorg-x11-xauth",
			"metacity",
			"xrdb",
			"xz",
		},
	})

	if common.VersionLessThan(t.arch.distro.osVersion, "39") {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"lklug-fonts", // orphaned, unavailable in F39
			},
		})
	}

	switch t.Arch().Name() {
	case platform.ARCH_X86_64.String():
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"biosdevname",
				"dmidecode",
				"grub2-tools-efi",
				"memtest86+",
			},
		})

	case platform.ARCH_AARCH64.String():
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"dmidecode",
			},
		})

	default:
		panic(fmt.Sprintf("unsupported arch: %s", t.Arch().Name()))
	}

	return ps
}

func iotInstallerPackageSet(t *imageType) rpmmd.PackageSet {
	// include anaconda packages
	ps := anacondaPackageSet(t)

	releasever := t.Arch().Distro().Releasever()
	version, err := strconv.Atoi(releasever)
	if err != nil {
		panic("cannot convert releasever to int: " + err.Error())
	}

	if version >= 38 {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"fedora-release-iot",
			},
		})
	}

	return ps
}

func liveInstallerPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"@workstation-product-environment",
			"@anaconda-tools",
			"anaconda-install-env-deps",
			"anaconda-live",
			"anaconda-dracut",
			"dracut-live",
			"glibc-all-langpacks",
			"kernel",
			"kernel-modules",
			"kernel-modules-extra",
			"livesys-scripts",
			"rng-tools",
			"rdma-core",
			"gnome-kiosk",
		},
		Exclude: []string{
			"@dial-up",
			"@input-methods",
			"@standard",
			"device-mapper-multipath",
			"fcoe-utils",
			"gfs2-utils",
			"reiserfs-utils",
			"sdubby",
		},
	}

	// We want to generate a preview image when rawhide is built
	if !common.VersionLessThan(t.arch.distro.osVersion, "39") {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"anaconda-webui",
			},
		})
	}

	return ps
}

func imageInstallerPackageSet(t *imageType) rpmmd.PackageSet {
	ps := anacondaPackageSet(t)

	releasever := t.Arch().Distro().Releasever()
	version, err := strconv.Atoi(releasever)
	if err != nil {
		panic("cannot convert releasever to int: " + err.Error())
	}

	// We want to generate a preview image when rawhide is built
	if version >= 38 {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"anaconda-webui",
			},
		})
	}

	return ps
}

func containerPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"bash",
			"coreutils",
			"dnf-yum",
			"dnf",
			"fedora-release-container",
			"glibc-minimal-langpack",
			"rootfiles",
			"rpm",
			"sudo",
			"tar",
			"util-linux-core",
			"vim-minimal",
		},
		Exclude: []string{
			"crypto-policies-scripts",
			"dbus-broker",
			"deltarpm",
			"dosfstools",
			"e2fsprogs",
			"elfutils-debuginfod-client",
			"fuse-libs",
			"gawk-all-langpacks",
			"glibc-gconv-extra",
			"glibc-langpack-en",
			"gnupg2-smime",
			"grubby",
			"kernel-core",
			"kernel-debug-core",
			"kernel",
			"langpacks-en_GB",
			"langpacks-en",
			"libss",
			"libxcrypt-compat",
			"nano",
			"openssl-pkcs11",
			"pinentry",
			"python3-unbound",
			"shared-mime-info",
			"sssd-client",
			"sudo-python-plugin",
			"systemd",
			"trousers",
			"whois-nls",
			"xkeyboard-config",
		},
	}

	if common.VersionLessThan(t.arch.distro.osVersion, "39") {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"fedora-repos-modular",
			},
		})
	}

	return ps
}

func minimalrpmPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core",
			"initial-setup",
			"libxkbcommon",
			"NetworkManager-wifi",
			"brcmfmac-firmware",
			"realtek-firmware",
			"iwlwifi-mvm-firmware",
		},
	}
}

func iotSimplifiedInstallerPackageSet(t *imageType) rpmmd.PackageSet {
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
			"fedora-logos",
			"gdisk",
			"gzip",
			"ima-evm-utils",
			"iproute",
			"iptables",
			"iputils",
			"iscsi-initiator-utils",
			"keyutils",
			"lldpad",
			"lvm2",
			"mdadm",
			"nss-softokn",
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

	return ps
}
