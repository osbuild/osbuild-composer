package fedora

// This file defines package sets that are used by more than one image type.

import (
	"fmt"
	"strconv"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
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
	ps := rpmmd.PackageSet{
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

	return ps
}

func vmdkCommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
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

	return ps
}

// fedora iot commit OS package set
func iotCommitPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"attr",
			"audit",
			"basesystem",
			"bash",
			"bash-completion",
			"bluez",
			"bluez-libs",
			"bluez-mesh",
			"chrony",
			"clevis",
			"clevis-dracut",
			"clevis-luks",
			"clevis-pin-tpm2",
			"container-selinux",
			"coreutils",
			"cracklib-dicts",
			"criu",
			"cryptsetup",
			"curl",
			"dbus-parsec",
			"dnsmasq",
			"dosfstools",
			"dracut-network",
			"e2fsprogs",
			"fedora-iot-config",
			"fedora-release-iot",
			"firewalld",
			"fuse-overlayfs",
			"fwupd",
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
			"iptables",
			"iputils",
			"iwd",
			"iwl7260-firmware",
			"iwlax2xx-firmware",
			"kernel-tools",
			"keyutils",
			"less",
			"libgpiod-utils",
			"libsss_sudo",
			"lvm2",
			"NetworkManager",
			"NetworkManager-wifi",
			"NetworkManager-wwan",
			"nss-altfiles",
			"openssh-clients",
			"openssh-server",
			"parsec",
			"passwd",
			"pinentry",
			"podman",
			"policycoreutils",
			"policycoreutils-python-utils",
			"polkit",
			"procps-ng",
			"python3",
			"rng-tools",
			"rootfiles",
			"rpm",
			"rsync",
			"screen",
			"setools-console",
			"setup",
			"shadow-utils",
			"shadow-utils",
			"skopeo",
			"slirp4netns",
			"smartmontools-selinux",
			"sssd-client",
			"sudo",
			"systemd",
			"tar",
			"tmux",
			"tpm2-pkcs11",
			"traceroute",
			"usbguard",
			"util-linux",
			"vim-minimal",
			"which",
			"wpa_supplicant",
			"xfsprogs",
			"xz",
			"zezere-ignition",
		},
	}

	switch t.arch.Name() {
	case platform.ARCH_X86_64.String():
		ps = ps.Append(x8664IotCommitPackageSet(t))
	case platform.ARCH_AARCH64.String():
		ps = ps.Append(aarch64IotCommitPackageSet(t))
	default:
		panic(fmt.Sprintf("unsupported arch: %s", t.arch.Name()))
	}

	if !common.VersionLessThan(t.arch.distro.osVersion, "38") {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"fdo-client",
				"fdo-owner-cli",
				"ignition-edge",
				"ssh-key-dir",
			},
		},
		)
	}
	return ps

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
			"rng-tools",
			"rpcbind",
			"selinux-policy-targeted",
			"systemd",
			"tar",
			"xfsprogs",
			"xz",
		},
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

func x8664IotCommitPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"biosdevname",
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

func aarch64IotCommitPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"grub2-efi-aa64",
			"efibootmgr",
			"shim-aa64",
			"iwl7260-firmware",
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

	switch t.arch.Name() {

	case platform.ARCH_X86_64.String():
		ps = ps.Append(x8664IotCommitPackageSet(t))
	case platform.ARCH_AARCH64.String():
		ps = ps.Append(aarch64IotCommitPackageSet(t))

	default:
		panic(fmt.Sprintf("unsupported arch: %s", t.arch.Name()))
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
			"fedora-repos-modular",
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

	return ps
}

func minimalrpmPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core",
		},
	}
}
