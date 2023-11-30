package rhel8

import (
	"fmt"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/rpmmd"
)

func imageInstaller() imageType {
	return imageType{
		name:     "image-installer",
		filename: "installer.iso",
		mimeType: "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			osPkgsKey:        bareMetalPackageSet,
			installerPkgsKey: anacondaPackageSet,
		},
		rpmOstree:        false,
		bootISO:          true,
		bootable:         true,
		image:            imageInstallerImage,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "rootfs-image", "efiboot-tree", "os", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}
}

func tarImgType() imageType {
	return imageType{
		name:     "tar",
		filename: "root.tar.xz",
		mimeType: "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: func(t *imageType) rpmmd.PackageSet {
				return rpmmd.PackageSet{
					Include: []string{"policycoreutils", "selinux-policy-targeted"},
					Exclude: []string{"rng-tools"},
				}
			},
		},
		image:            tarImage,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "archive"},
		exports:          []string{"archive"},
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
	}.Append(distroSpecificPackageSet(t))

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
	case arch.ARCH_X86_64.String():
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
	case arch.ARCH_X86_64.String():
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"biosdevname",
				"dmidecode",
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
		panic(fmt.Sprintf("unsupported arch: %s", t.arch.Name()))
	}

	return ps
}
