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
			Include: []string{
				"rpm-ostree",
			},
			Exclude: nil,
		})
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

func edgeSimplifiedInstallerBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return edgeInstallerBuildPackageSet(t).Append(
		edgeEncryptionBuildPackageSet(t),
	)
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

// OS package sets

func qcow2CommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"@core",
			"authselect-compat",
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"cockpit-system",
			"cockpit-ws",
			"dhcp-client",
			"dnf",
			"dnf-utils",
			"dosfstools",
			"dracut-norescue",
			"net-tools",
			"NetworkManager",
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
			"yum",
		},
		Exclude: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-lib",
			"alsa-tools-firmware",
			"biosdevname",
			"dnf-plugin-spacewalk",
			"dracut-config-rescue",
			"fedora-release",
			"fedora-repos",
			"firewalld",
			"fwupd",
			"iprutils",
			"ivtv-firmware",
			"iwl1000-firmware",
			"iwl100-firmware",
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
			"langpacks-*",
			"langpacks-en",
			"langpacks-en",
			"libertas-sd8686-firmware",
			"libertas-sd8787-firmware",
			"libertas-usb8388-firmware",
			"nss",
			"plymouth",
			"rng-tools",
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
			"@Core",
			"langpacks-en",

			// From the lorax kickstart
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"gdisk",
			"net-tools",
			"python3",
			"selinux-policy-targeted",
			"WALinuxAgent",

			// removed from defaults but required to boot in azure
			"dhcp-client",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"rng-tools",
		},
	}.Append(bootPackageSet(t))
}

func azureRhuiCommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@Server",
			"NetworkManager",
			"NetworkManager-cloud-setup",
			"kernel",
			"kernel-core",
			"kernel-modules",
			"selinux-policy-targeted",
			"efibootmgr",
			"lvm2",
			"grub2-efi-x64",
			"shim-x64",
			"dracut-config-generic",
			"dracut-norescue",
			"bzip2",
			"langpacks-en",
			"grub2-pc",
			"rhc",
			"yum-utils",
			"rhui-azure-rhel8",
			"WALinuxAgent",
			"cloud-init",
			"cloud-utils-growpart",
			"gdisk",
			"hyperv-daemons",
			"nvme-cli",
			"cryptsetup-reencrypt",
			"uuid",
			"rng-tools",
			"patch",
		},
		Exclude: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-lib",
			"alsa-sof-firmware",
			"alsa-tools-firmware",
			"dracut-config-rescue",
			"ivtv-firmware",
			"iwl1000-firmware",
			"iwl100-firmware",
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
			"glibc-all-langpacks",
			"biosdevname",
			"cockpit-podman",
			"bolt",
			"buildah",
			"containernetworking-plugins",
			"dnf-plugin-spacewalk",
			"iprutils",
			"plymouth",
			"podman",
			"python3-dnf-plugin-spacewalk",
			"python3-rhnlib",
			"python3-hwdata",
			"NetworkManager-config-server",
			"rhn-client-tools",
			"rhn-setup",
			"rhnsd",
			"rhn-check",
			"rhnlib",
			"usb_modeswitch",
		},
	}.Append(bootPackageSet(t))
}

func vmdkCommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core",
			"chrony",
			"cloud-init",
			"firewalld",
			"langpacks-en",
			"open-vm-tools",
			"selinux-policy-targeted",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"rng-tools",
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
			"@core",
			"authselect-compat",
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"dhcp-client",
			"dracut-config-generic",
			"dracut-norescue",
			"gdisk",
			"grub2",
			"langpacks-en",
			"NetworkManager",
			"NetworkManager-cloud-setup",
			"redhat-release",
			"redhat-release-eula",
			"rsync",
			"tar",
			"yum-utils",
		},
		Exclude: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-tools-firmware",
			"biosdevname",
			"firewalld",
			"iprutils",
			"ivtv-firmware",
			"iwl1000-firmware",
			"iwl100-firmware",
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
			"plymouth",
			// RHBZ#2075815
			"qemu-guest-agent",
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
		// RHBZ#2074107
		"@Server",
		// SAP System Roles
		// https://access.redhat.com/sites/default/files/attachments/rhel_system_roles_for_sap_1.pdf
		"rhel-system-roles-sap",
		// RHBZ#1959813
		"bind-utils",
		"compat-sap-c++-9",
		"compat-sap-c++-10", // RHBZ#2074114
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

	if t.arch.distro.osVersion == "8.4" {
		ec2SapPackageSet = ec2SapPackageSet.Append(rpmmd.PackageSet{
			Include: []string{"ansible"},
		})
	} else {
		// 8.6+ and CS8 (image type does not exist on 8.5)
		ec2SapPackageSet = ec2SapPackageSet.Append(rpmmd.PackageSet{
			Include: []string{"ansible-core"}, // RHBZ#2077356
		})
	}
	return ec2SapPackageSet
}

// common GCE image
func gceCommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core",
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
	}.Append(bootPackageSet(t)).Append(distroSpecificPackageSet(t))
}

// GCE BYOS image
func gcePackageSet(t *imageType) rpmmd.PackageSet {
	return gceCommonPackageSet(t)
}

// GCE RHUI image
func gceRhuiPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"google-rhui-client-rhel8",
		},
	}.Append(gceCommonPackageSet(t))
}

// edge commit OS package set
func edgeCommitPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"attr",
			"audit",
			"basesystem",
			"bash",
			"bash-completion",
			"chrony",
			"clevis",
			"clevis-dracut",
			"clevis-luks",
			"container-selinux",
			"coreutils",
			"criu",
			"cryptsetup",
			"curl",
			"dnsmasq",
			"dosfstools",
			"dracut-config-generic",
			"dracut-network",
			"e2fsprogs",
			"firewalld",
			"fuse-overlayfs",
			"fwupd",
			"glibc",
			"glibc-minimal-langpack",
			"gnupg2",
			"greenboot",
			"gzip",
			"hostname",
			"ima-evm-utils",
			"iproute",
			"iptables",
			"iputils",
			"keyutils",
			"less",
			"lvm2",
			"NetworkManager",
			"NetworkManager-wifi",
			"NetworkManager-wwan",
			"nss-altfiles",
			"openssh-clients",
			"openssh-server",
			"passwd",
			"pinentry",
			"platform-python",
			"podman",
			"policycoreutils",
			"policycoreutils-python-utils",
			"polkit",
			"procps-ng",
			"redhat-release",
			"rootfiles",
			"rpm",
			"rpm-ostree",
			"rsync",
			"selinux-policy-targeted",
			"setools-console",
			"setup",
			"shadow-utils",
			"shadow-utils",
			"skopeo",
			"slirp4netns",
			"sudo",
			"systemd",
			"tar",
			"tmux",
			"traceroute",
			"usbguard",
			"util-linux",
			"vim-minimal",
			"wpa_supplicant",
			"xz",
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

	if t.arch.distro.isRHEL() && versionLessThan(t.arch.distro.osVersion, "8.6") {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"greenboot-grub2",
				"greenboot-reboot",
				"greenboot-rpm-ostree-grub2",
				"greenboot-status",
			},
		})
	} else {
		// 8.6+ and CS8
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"fdo-client",
				"fdo-owner-cli",
				"greenboot-default-health-checks",
			},
		})
	}

	return ps

}

func x8664EdgeCommitPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"efibootmgr",
			"grub2",
			"grub2-efi-x64",
			"iwl1000-firmware",
			"iwl100-firmware",
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
			"microcode_ctl",
			"shim-x64",
		},
		Exclude: nil,
	}
}

func aarch64EdgeCommitPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"efibootmgr",
			"grub2-efi-aa64",
			"iwl7260-firmware",
			"shim-aa64",
		},
		Exclude: nil,
	}
}

func bareMetalPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"@core",
			"authselect-compat",
			"chrony",
			"cockpit-system",
			"cockpit-ws",
			"dhcp-client",
			"dnf",
			"dnf-utils",
			"dosfstools",
			"dracut-norescue",
			"iwl1000-firmware",
			"iwl100-firmware",
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
			"lvm2",
			"net-tools",
			"NetworkManager",
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
			"selinux-policy-targeted",
			"tar",
			"tcpdump",
			"yum",
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
			"metacity",
			"mtr",
			"mt-st",
			"net-tools",
			"nmap-ncat",
			"nm-connection-editor",
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
			"xorg-x11-server-utils",
			"xorg-x11-server-Xorg",
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
