package fedora33

// This file defines package sets that are used by more than one image type.

import (
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
			"policycoreutils",
			"qemu-img",
			"selinux-policy-targeted",
			"systemd",
			"tar",
			"xz",
		},
	}

	if t.arch.Name() == distro.X86_64ArchName {
		ps = ps.Append(x8664BuildPackageSet(t))
	}
	return ps
}

// x86_64 build package set
func x8664BuildPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{"grub2-pc"},
	}
}

// AMI type
func ec2CorePackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@Core",
			"chrony",
			"selinux-policy-targeted",
			"langpacks-en",
			"libxcrypt-compat",
			"xfsprogs",
			"cloud-init",
			"checkpolicy",
			"net-tools",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"geolite2-city",
			"geolite2-country",
			"zram-generator-defaults",
		},
	}
}

// QCOW2 type
func qcow2PackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@Fedora Cloud Server",
			"chrony",
			"systemd-udev",
			"selinux-policy-targeted",
			"langpacks-en",
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
		},
	}
}

// OpenStack package set
func openStackPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@Core",
			"chrony",
			"selinux-policy-targeted",
			"spice-vdagent",
			"qemu-guest-agent",
			"xen-libs",
			"langpacks-en",
			"cloud-init",
			"libdrm",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"geolite2-city",
			"geolite2-country",
			"zram-generator-defaults",
		},
	}
}

// VHD package set
func vhdPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@Core",
			"chrony",
			"selinux-policy-targeted",
			"langpacks-en",
			"net-tools",
			"ntfsprogs",
			"WALinuxAgent",
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

// VMDK package set
func vmdkPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@Fedora Cloud Server",
			"chrony",
			"systemd-udev",
			"selinux-policy-targeted",
			"langpacks-en",
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
		},
	}
}

// OCI Image package set
func ociImagePackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@Fedora Cloud Server",
			"chrony",
			"systemd-udev",
			"selinux-policy-targeted",
			"langpacks-en",
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
		},
	}
}

// iot commit OS package set

// common iot image build package set
func iotBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return distroBuildPackageSet(t).Append(rpmmd.PackageSet{Include: []string{"rpm-ostree"}})
}

func iotCommitPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"fedora-release-iot",
			"glibc", "glibc-minimal-langpack", "nss-altfiles",
			"sssd-client", "libsss_sudo", "shadow-utils",
			"dracut-config-generic", "dracut-network", "polkit", "lvm2",
			"cryptsetup", "pinentry",
			"keyutils", "cracklib-dicts",
			"e2fsprogs", "xfsprogs", "dosfstools",
			"gnupg2",
			"basesystem", "python3", "bash",
			"xz", "gzip",
			"coreutils", "which", "curl",
			"firewalld", "iptables",
			"NetworkManager", "NetworkManager-wifi", "NetworkManager-wwan",
			"wpa_supplicant", "iwd", "tpm2-pkcs11",
			"dnsmasq", "traceroute",
			"hostname", "iproute", "iputils",
			"openssh-clients", "openssh-server", "passwd",
			"policycoreutils", "procps-ng", "rootfiles", "rpm",
			"selinux-policy-targeted", "setup", "shadow-utils",
			"sudo", "systemd", "util-linux", "vim-minimal",
			"less", "tar",
			"fwupd", "usbguard",
			"greenboot", "greenboot-grub2", "greenboot-rpm-ostree-grub2", "greenboot-reboot", "greenboot-status",
			"ignition", "zezere-ignition",
			"rsync", "attr",
			"ima-evm-utils",
			"bash-completion",
			"tmux", "screen",
			"policycoreutils-python-utils",
			"setools-console",
			"audit", "rng-tools", "chrony",
			"bluez", "bluez-libs", "bluez-mesh",
			"kernel-tools", "libgpiod-utils",
			"podman", "container-selinux", "skopeo", "criu",
			"slirp4netns", "fuse-overlayfs",
			"clevis", "clevis-dracut", "clevis-luks", "clevis-pin-tpm2",
			"parsec", "dbus-parsec",
			"iwl7260-firmware", "iwlax2xx-firmware",
		},
	}
	switch t.arch.Name() {
	case distro.X86_64ArchName:
		ps = ps.Append(x8664IOTCommitPackageSet())

	case distro.Aarch64ArchName:
		ps = ps.Append(aarch64IOTCommitPackageSet())
	}

	return ps

}

func x8664IOTCommitPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"grub2", "grub2-efi-x64", "efibootmgr", "shim-x64", "microcode_ctl",
			"iwl1000-firmware", "iwl100-firmware", "iwl105-firmware", "iwl135-firmware",
			"iwl2000-firmware", "iwl2030-firmware", "iwl3160-firmware", "iwl5000-firmware",
			"iwl5150-firmware", "iwl6000-firmware", "iwl6050-firmware",
		},
	}
}

func aarch64IOTCommitPackageSet() rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{"grub2", "grub2-efi-aa64", "efibootmgr", "shim-aa64", "uboot-images-armv8",
			"bcm283x-firmware", "arm-image-installer"},
	}
}
