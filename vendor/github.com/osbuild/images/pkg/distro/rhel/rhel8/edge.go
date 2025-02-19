package rhel8

import (
	"fmt"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
)

func mkEdgeCommitImgType(rd *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-commit",
		"commit.tar",
		"application/x-tar",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: edgeCommitPackageSet,
		},
		rhel.EdgeCommitImage,
		[]string{"build"},
		[]string{"os", "ostree-commit", "commit-archive"},
		[]string{"commit-archive"},
	)

	it.NameAliases = []string{"rhel-edge-commit"}
	it.DefaultImageConfig = &distro.ImageConfig{
		EnabledServices: edgeServices(rd),
		DracutConf:      []*osbuild.DracutConfStageOptions{osbuild.FIPSDracutConfStageOptions},
	}
	it.RPMOSTree = true

	return it
}

func mkEdgeOCIImgType(rd *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-container",
		"container.tar",
		"application/x-tar",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: edgeCommitPackageSet,
			rhel.ContainerPkgsKey: func(t *rhel.ImageType) rpmmd.PackageSet {
				return rpmmd.PackageSet{
					Include: []string{"nginx"},
				}
			},
		},
		rhel.EdgeContainerImage,
		[]string{"build"},
		[]string{"os", "ostree-commit", "container-tree", "container"},
		[]string{"container"},
	)

	it.NameAliases = []string{"rhel-edge-container"}
	it.DefaultImageConfig = &distro.ImageConfig{
		EnabledServices: edgeServices(rd),
		DracutConf:      []*osbuild.DracutConfStageOptions{osbuild.FIPSDracutConfStageOptions},
	}
	it.RPMOSTree = true

	return it
}

func mkEdgeRawImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-raw-image",
		"image.raw.xz",
		"application/xz",
		nil,
		rhel.EdgeRawImage,
		[]string{"build"},
		[]string{"ostree-deployment", "image", "xz"},
		[]string{"xz"},
	)

	it.NameAliases = []string{"rhel-edge-raw-image"}
	it.Compression = "xz"
	it.KernelOptions = "modprobe.blacklist=vc4"
	it.DefaultImageConfig = &distro.ImageConfig{
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
		},
		Locale:       common.ToPtr("C.UTF-8"),
		LockRootUser: common.ToPtr(true),
	}
	it.DefaultSize = 10 * datasizes.GibiByte
	it.RPMOSTree = true
	it.Bootable = true
	it.BasePartitionTables = edgeBasePartitionTables
	it.UnsupportedPartitioningModes = []disk.PartitioningMode{
		disk.AutoLVMPartitioningMode,
		disk.LVMPartitioningMode,
	}

	return it
}

func mkEdgeInstallerImgType(rd *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-installer",
		"installer.iso",
		"application/x-iso9660-image",
		map[string]rhel.PackageSetFunc{
			// TODO: non-arch-specific package set handling for installers
			// This image type requires build packages for installers and
			// ostree/edge.  For now we only have x86-64 installer build
			// package sets defined.  When we add installer build package sets
			// for other architectures, this will need to be moved to the
			// architecture and the merging will happen in the PackageSets()
			// method like the other sets.
			rhel.InstallerPkgsKey: edgeInstallerPackageSet,
		},
		rhel.EdgeInstallerImage,
		[]string{"build"},
		[]string{"anaconda-tree", "rootfs-image", "efiboot-tree", "bootiso-tree", "bootiso"},
		[]string{"bootiso"},
	)

	it.NameAliases = []string{"rhel-edge-installer"}
	it.DefaultImageConfig = &distro.ImageConfig{
		EnabledServices: edgeServices(rd),
	}
	it.DefaultInstallerConfig = &distro.InstallerConfig{
		AdditionalDracutModules: []string{
			"prefixdevname",
			"prefixdevname-tools",
			"ifcfg",
		},
	}
	it.RPMOSTree = true
	it.BootISO = true
	it.ISOLabelFn = distroISOLabelFunc

	return it
}

func mkEdgeSimplifiedInstallerImgType(rd *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-simplified-installer",
		"simplified-installer.iso",
		"application/x-iso9660-image",
		map[string]rhel.PackageSetFunc{
			// TODO: non-arch-specific package set handling for installers
			// This image type requires build packages for installers and
			// ostree/edge.  For now we only have x86-64 installer build
			// package sets defined.  When we add installer build package sets
			// for other architectures, this will need to be moved to the
			// architecture and the merging will happen in the PackageSets()
			// method like the other sets.
			rhel.InstallerPkgsKey: edgeSimplifiedInstallerPackageSet,
		},
		rhel.EdgeSimplifiedInstallerImage,
		[]string{"build"},
		[]string{"ostree-deployment", "image", "xz", "coi-tree", "efiboot-tree", "bootiso-tree", "bootiso"},
		[]string{"bootiso"},
	)

	it.NameAliases = []string{"rhel-edge-simplified-installer"}
	it.KernelOptions = "modprobe.blacklist=vc4"
	it.DefaultImageConfig = &distro.ImageConfig{
		EnabledServices: edgeServices(rd),
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
		},
		Locale:       common.ToPtr("C.UTF-8"),
		LockRootUser: common.ToPtr(true),
	}
	it.DefaultInstallerConfig = &distro.InstallerConfig{
		AdditionalDracutModules: []string{
			"prefixdevname",
			"prefixdevname-tools",
		},
	}
	it.DefaultSize = 10 * datasizes.GibiByte
	it.RPMOSTree = true
	it.Bootable = true
	it.BootISO = true
	it.ISOLabelFn = distroISOLabelFunc
	it.BasePartitionTables = edgeBasePartitionTables
	it.UnsupportedPartitioningModes = []disk.PartitioningMode{
		disk.AutoLVMPartitioningMode,
		disk.LVMPartitioningMode,
	}

	return it
}

func mkMinimalRawImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"minimal-raw",
		"disk.raw.xz",
		"application/xz",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: minimalrpmPackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.DefaultImageConfig = &distro.ImageConfig{
		EnabledServices: minimalrawServices,
		// NOTE: temporary workaround for a bug in initial-setup that
		// requires a kickstart file in the root directory.
		Files: []*fsnode.File{initialSetupKickstart()},
	}
	it.KernelOptions = "ro"
	it.Bootable = true
	it.DefaultSize = 2 * datasizes.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// edge commit OS package set
func edgeCommitPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
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

	switch t.Arch().Name() {
	case arch.ARCH_X86_64.String():
		ps = ps.Append(x8664EdgeCommitPackageSet(t))

	case arch.ARCH_AARCH64.String():
		ps = ps.Append(aarch64EdgeCommitPackageSet(t))
	}

	if t.IsRHEL() && common.VersionLessThan(t.Arch().Distro().OsVersion(), "8.6") {
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
				"sos",
			},
		})
	}

	return ps

}

func x8664EdgeCommitPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
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

func aarch64EdgeCommitPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
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

func edgeInstallerPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	return anacondaPackageSet(t)
}

func edgeSimplifiedInstallerPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
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
			"redhat-logos",
			"rootfiles",
			"setools-console",
			"sudo",
			"traceroute",
			"util-linux",
		},
		Exclude: nil,
	})

	switch t.Arch().Name() {

	case arch.ARCH_X86_64.String():
		ps = ps.Append(x8664EdgeCommitPackageSet(t))
	case arch.ARCH_AARCH64.String():
		ps = ps.Append(aarch64EdgeCommitPackageSet(t))

	default:
		panic(fmt.Sprintf("unsupported arch: %s", t.Arch().Name()))
	}

	return ps
}

func edgeServices(rd *rhel.Distribution) []string {
	// Common Services
	var edgeServices = []string{"NetworkManager.service", "firewalld.service", "sshd.service"}

	if rd.OsVersion() == "8.4" {
		// greenboot services aren't enabled by default in 8.4
		edgeServices = append(edgeServices,
			"greenboot-grub2-set-counter",
			"greenboot-grub2-set-success",
			"greenboot-healthcheck",
			"greenboot-rpm-ostree-grub2-check-fallback",
			"greenboot-status",
			"greenboot-task-runner",
			"redboot-auto-reboot",
			"redboot-task-runner")

	}

	if !(rd.IsRHEL() && common.VersionLessThan(rd.OsVersion(), "8.6")) {
		// enable fdo-client only on RHEL 8.6+ and CS8

		// TODO(runcom): move fdo-client-linuxapp.service to presets?
		edgeServices = append(edgeServices, "fdo-client-linuxapp.service")
	}

	return edgeServices
}

var minimalrawServices = []string{
	"NetworkManager.service",
	"firewalld.service",
	"sshd.service",
	"initial-setup.service",
}

// initialSetupKickstart returns the File configuration for a kickstart file
// that's required to enable initial-setup to run on first boot.
func initialSetupKickstart() *fsnode.File {
	file, err := fsnode.NewFile("/root/anaconda-ks.cfg", nil, "root", "root", []byte("# Run initial-setup on first boot\n# Created by osbuild\nfirstboot --reconfig\nlang en_US.UTF-8\n"))
	if err != nil {
		panic(err)
	}
	return file
}
