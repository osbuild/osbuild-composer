package rhel10

import (
	"fmt"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/rpmmd"
)

func mkTarImgType() *rhel.ImageType {
	return rhel.NewImageType(
		"tar",
		"root.tar.xz",
		"application/x-tar",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: func(t *rhel.ImageType) rpmmd.PackageSet {
				return rpmmd.PackageSet{
					Include: []string{"policycoreutils", "selinux-policy-targeted"},
					Exclude: []string{"rng-tools"},
				}
			},
		},
		rhel.TarImage,
		[]string{"build"},
		[]string{"os", "archive"},
		[]string{"archive"},
	)
}

func mkImageInstallerImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"image-installer",
		"installer.iso",
		"application/x-iso9660-image",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey:        bareMetalPackageSet,
			rhel.InstallerPkgsKey: anacondaPackageSet,
		},
		rhel.ImageInstallerImage,
		[]string{"build"},
		[]string{"anaconda-tree", "efiboot-tree", "os", "bootiso-tree", "bootiso"},
		[]string{"bootiso"},
	)

	it.BootISO = true
	it.Bootable = true
	it.ISOLabelFn = distroISOLabelFunc

	it.DefaultInstallerConfig = &distro.InstallerConfig{
		AdditionalDracutModules: []string{
			"nvdimm", // non-volatile DIMM firmware (provides nfit, cuse, and nd_e820)
			"prefixdevname",
			"prefixdevname-tools",
		},
		AdditionalDrivers: []string{
			"ipmi_devintf",
			"ipmi_msghandler",
		},
	}

	return it
}

// PACKAGE SETS

func bareMetalPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"@core",
			"chrony",
			"cockpit-system",
			"cockpit-ws",
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
			"tuned",
		},
		Exclude: []string{
			"dracut-config-rescue",
		},
	}.Append(distroBuildPackageSet(t))

	// Ensure to not pull in subscription-manager on non-RHEL distro
	if t.IsRHEL() {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"subscription-manager-cockpit",
			},
		})
	}

	return ps
}

func installerPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
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

	ps = ps.Append(rpmmd.PackageSet{
		// Extra packages that are required by the dracut stage of all installers.
		// These are weak deps of other packages in the list above, but lets be
		// explicit about what we need and not rely on the weak deps. Relying
		// on hard-dependencies for other modules is OK for now.
		//
		// TODO: add these dynamically based on the modules enabled by each
		// pipeline.
		Include: []string{
			"mdadm",
			"nss-softokn",
		},
	})

	switch t.Arch().Name() {
	case arch.ARCH_X86_64.String():
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"biosdevname",
			},
		})
	}

	return ps
}

func anacondaPackageSet(t *rhel.ImageType) rpmmd.PackageSet {

	// common installer packages
	ps := installerPackageSet(t)

	ps = ps.Append(rpmmd.PackageSet{
		Include: []string{
			"alsa-firmware",
			"alsa-tools-firmware",
			"anaconda",
			"anaconda-dracut",
			"anaconda-install-img-deps",
			"anaconda-widgets",
			"audit",
			"bind-utils",
			"bzip2",
			"cryptsetup",
			"curl",
			"dbus-x11",
			"default-fonts-core-sans",
			"default-fonts-other-sans",
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
			"kbd",
			"kbd-misc",
			"kdump-anaconda-addon",
			"kernel",
			"less",
			"libblockdev-lvm-dbus",
			"libibverbs",
			"librsvg2",
			"linux-firmware",
			"lldpad",
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
			// the package is not yet available on c10s / el10
			//"oscap-anaconda-addon",
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
			"sil-padauk-fonts",
			"smartmontools",
			"spice-vdagent",
			"strace",
			"systemd",
			"tar",
			"udisks2",
			"udisks2-iscsi",
			"usbutils",
			"vim-minimal",
			"volume_key",
			"wget",
			"xfsdump",
			"xfsprogs",
			"xz",
		},
	})

	ps = ps.Append(anacondaBootPackageSet(t))

	switch t.Arch().Name() {
	case arch.ARCH_X86_64.String():
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"biosdevname",
				"dmidecode",
				"grub2-tools-efi",
				"memtest86+",
			},
		})

	case arch.ARCH_AARCH64.String():
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
