package fedora

// This file defines package sets that are used by more than one image type.

import (
	"fmt"
	"strconv"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

func qcow2CommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@Fedora Cloud Server",
			"chrony", // not mentioned in the kickstart, anaconda pulls it when setting the timezone
			"langpacks-en",
			"qemu-guest-agent",
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

func vhdCommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core",
			"chrony",
			"langpacks-en",
			"net-tools",
			"ntfsprogs",
			"libxcrypt-compat",
			"initscripts",
			"glibc-all-langpacks",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"geolite2-city",
			"geolite2-country",
			"zram-generator-defaults",
		},
	}
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
	return rpmmd.PackageSet{
		Include: []string{
			"fedora-release-iot",
			"glibc",
			"glibc-minimal-langpack",
			"nss-altfiles",
			"sssd-client",
			"libsss_sudo",
			"shadow-utils",
			"dracut-network",
			"polkit",
			"lvm2",
			"cryptsetup",
			"pinentry",
			"keyutils",
			"cracklib-dicts",
			"e2fsprogs",
			"xfsprogs",
			"dosfstools",
			"gnupg2",
			"basesystem",
			"python3",
			"bash",
			"xz",
			"gzip",
			"coreutils",
			"which",
			"curl",
			"firewalld",
			"iptables",
			"NetworkManager",
			"NetworkManager-wifi",
			"NetworkManager-wwan",
			"wpa_supplicant",
			"iwd",
			"tpm2-pkcs11",
			"dnsmasq",
			"traceroute",
			"hostname",
			"iproute",
			"iputils",
			"openssh-clients",
			"openssh-server",
			"passwd",
			"policycoreutils",
			"procps-ng",
			"rootfiles",
			"rpm",
			"smartmontools-selinux",
			"setup",
			"shadow-utils",
			"sudo",
			"systemd",
			"util-linux",
			"vim-minimal",
			"less",
			"tar",
			"fwupd",
			"usbguard",
			"greenboot",
			"ignition",
			"zezere-ignition",
			"rsync",
			"attr",
			"ima-evm-utils",
			"bash-completion",
			"tmux",
			"screen",
			"policycoreutils-python-utils",
			"setools-console",
			"audit",
			"rng-tools",
			"chrony",
			"bluez",
			"bluez-libs",
			"bluez-mesh",
			"kernel-tools",
			"libgpiod-utils",
			"podman",
			"container-selinux",
			"skopeo",
			"criu",
			"slirp4netns",
			"fuse-overlayfs",
			"clevis",
			"clevis-dracut",
			"clevis-luks",
			"clevis-pin-tpm2",
			"parsec",
			"dbus-parsec",
			"iwl7260-firmware",
			"iwlax2xx-firmware",
			"greenboot-default-health-checks",
		},
	}
}

// INSTALLER PACKAGE SET

func installerPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
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
		},
	}
}
