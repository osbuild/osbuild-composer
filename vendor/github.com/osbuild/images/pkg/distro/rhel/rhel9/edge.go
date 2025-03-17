package rhel9

import (
	"fmt"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
)

func mkEdgeCommitImgType(d *rhel.Distribution) *rhel.ImageType {
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
	it.RPMOSTree = true

	it.DefaultImageConfig = &distro.ImageConfig{
		EnabledServices: edgeServices,
		SystemdUnit:     systemdUnits,
	}
	if common.VersionGreaterThanOrEqual(d.OsVersion(), "9.2") || !d.IsRHEL() {
		it.DefaultImageConfig.EnabledServices = append(
			it.DefaultImageConfig.EnabledServices,
			"ignition-firstboot-complete.service",
			"coreos-ignition-write-issues.service",
		)
	}

	return it
}

func mkEdgeOCIImgType(d *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-container",
		"container.tar",
		"application/x-tar",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: edgeCommitPackageSet,
			rhel.ContainerPkgsKey: func(t *rhel.ImageType) rpmmd.PackageSet {
				return rpmmd.PackageSet{
					Include: []string{"nginx"}, // FIXME: this has no effect
				}
			},
		},
		rhel.EdgeContainerImage,
		[]string{"build"},
		[]string{"os", "ostree-commit", "container-tree", "container"},
		[]string{"container"},
	)

	it.NameAliases = []string{"rhel-edge-container"}
	it.RPMOSTree = true

	it.DefaultImageConfig = &distro.ImageConfig{
		EnabledServices: edgeServices,
		SystemdUnit:     systemdUnits,
	}
	if common.VersionGreaterThanOrEqual(d.OsVersion(), "9.2") || !d.IsRHEL() {
		it.DefaultImageConfig.EnabledServices = append(
			it.DefaultImageConfig.EnabledServices,
			"ignition-firstboot-complete.service",
			"coreos-ignition-write-issues.service",
		)
	}

	return it
}

func mkEdgeRawImgType(d *rhel.Distribution) *rhel.ImageType {
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
	it.DefaultImageConfig = &distro.ImageConfig{
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
		},
		Locale:       common.ToPtr("C.UTF-8"),
		LockRootUser: common.ToPtr(true),
	}
	if common.VersionGreaterThanOrEqual(d.OsVersion(), "9.2") || !d.IsRHEL() {
		it.DefaultImageConfig.OSTreeConfSysrootReadOnly = common.ToPtr(true)
		it.DefaultImageConfig.IgnitionPlatform = common.ToPtr("metal")
	}

	it.KernelOptions = []string{"modprobe.blacklist=vc4"}
	if common.VersionGreaterThanOrEqual(d.OsVersion(), "9.2") || !d.IsRHEL() {
		it.KernelOptions = append(it.KernelOptions, "rw", "coreos.no_persist_ip")
	}

	it.DefaultSize = 10 * datasizes.GibiByte
	it.RPMOSTree = true
	it.Bootable = true
	it.BasePartitionTables = edgeBasePartitionTables
	it.UnsupportedPartitioningModes = []disk.PartitioningMode{disk.RawPartitioningMode}

	return it
}

func mkEdgeInstallerImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-installer",
		"installer.iso",
		"application/x-iso9660-image",
		map[string]rhel.PackageSetFunc{
			rhel.InstallerPkgsKey: edgeInstallerPackageSet,
		},
		rhel.EdgeInstallerImage,
		[]string{"build"},
		[]string{"anaconda-tree", "rootfs-image", "efiboot-tree", "bootiso-tree", "bootiso"},
		[]string{"bootiso"},
	)

	it.NameAliases = []string{"rhel-edge-installer"}
	it.DefaultImageConfig = &distro.ImageConfig{
		Locale:          common.ToPtr("en_US.UTF-8"),
		EnabledServices: edgeServices,
	}
	it.DefaultInstallerConfig = &distro.InstallerConfig{
		AdditionalDracutModules: []string{
			"nvdimm", // non-volatile DIMM firmware (provides nfit, cuse, and nd_e820)
			"prefixdevname",
			"prefixdevname-tools",
			"ifcfg",
		},
		AdditionalDrivers: []string{
			"cuse",
			"ipmi_devintf",
			"ipmi_msghandler",
		},
	}
	it.RPMOSTree = true
	it.BootISO = true
	it.ISOLabelFn = distroISOLabelFunc

	return it
}

func mkEdgeSimplifiedInstallerImgType(d *rhel.Distribution) *rhel.ImageType {
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
	it.DefaultImageConfig = &distro.ImageConfig{
		EnabledServices: edgeServices,
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
		},
		Locale:       common.ToPtr("C.UTF-8"),
		LockRootUser: common.ToPtr(true),
	}
	if common.VersionGreaterThanOrEqual(d.OsVersion(), "9.2") || !d.IsRHEL() {
		it.DefaultImageConfig.OSTreeConfSysrootReadOnly = common.ToPtr(true)
		it.DefaultImageConfig.IgnitionPlatform = common.ToPtr("metal")
	}

	it.DefaultInstallerConfig = &distro.InstallerConfig{
		AdditionalDracutModules: []string{
			"prefixdevname",
			"prefixdevname-tools",
		},
	}

	it.DefaultSize = 10 * datasizes.GibiByte
	it.RPMOSTree = true
	it.BootISO = true
	it.Bootable = true
	it.ISOLabelFn = distroISOLabelFunc
	it.BasePartitionTables = edgeBasePartitionTables
	it.UnsupportedPartitioningModes = []disk.PartitioningMode{disk.RawPartitioningMode}

	it.KernelOptions = []string{"modprobe.blacklist=vc4"}
	if common.VersionGreaterThanOrEqual(d.OsVersion(), "9.2") || !d.IsRHEL() {
		it.KernelOptions = append(it.KernelOptions, "rw", "coreos.no_persist_ip")
	}

	return it
}

func mkEdgeAMIImgType(d *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-ami",
		"image.raw",
		"application/octet-stream",
		nil,
		rhel.EdgeRawImage,
		[]string{"build"},
		[]string{"ostree-deployment", "image"},
		[]string{"image"},
	)

	it.DefaultImageConfig = &distro.ImageConfig{
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
		},
		Locale:       common.ToPtr("C.UTF-8"),
		LockRootUser: common.ToPtr(true),
	}
	if common.VersionGreaterThanOrEqual(d.OsVersion(), "9.2") || !d.IsRHEL() {
		it.DefaultImageConfig.OSTreeConfSysrootReadOnly = common.ToPtr(true)
		it.DefaultImageConfig.IgnitionPlatform = common.ToPtr("metal")
	}

	it.KernelOptions = append(amiKernelOptions(), "modprobe.blacklist=vc4")
	if common.VersionGreaterThanOrEqual(d.OsVersion(), "9.2") || !d.IsRHEL() {
		it.KernelOptions = append(it.KernelOptions, "rw", "coreos.no_persist_ip")
	}

	it.DefaultSize = 10 * datasizes.GibiByte
	it.RPMOSTree = true
	it.Bootable = true
	it.BasePartitionTables = edgeBasePartitionTables
	it.UnsupportedPartitioningModes = []disk.PartitioningMode{disk.RawPartitioningMode}
	it.Environment = &environment.EC2{}

	return it
}

func mkEdgeVsphereImgType(d *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-vsphere",
		"image.vmdk",
		"application/x-vmdk",
		nil,
		rhel.EdgeRawImage,
		[]string{"build"},
		[]string{"ostree-deployment", "image", "vmdk"},
		[]string{"vmdk"},
	)

	it.DefaultImageConfig = &distro.ImageConfig{
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
		},
		Locale:       common.ToPtr("C.UTF-8"),
		LockRootUser: common.ToPtr(true),
	}
	if common.VersionGreaterThanOrEqual(d.OsVersion(), "9.2") || !d.IsRHEL() {
		it.DefaultImageConfig.OSTreeConfSysrootReadOnly = common.ToPtr(true)
		it.DefaultImageConfig.IgnitionPlatform = common.ToPtr("metal")
	}

	it.KernelOptions = []string{"modprobe.blacklist=vc4"}
	if common.VersionGreaterThanOrEqual(d.OsVersion(), "9.2") || !d.IsRHEL() {
		it.KernelOptions = append(it.KernelOptions, "rw", "coreos.no_persist_ip")
	}

	it.DefaultSize = 10 * datasizes.GibiByte
	it.RPMOSTree = true
	it.Bootable = true
	it.BasePartitionTables = edgeBasePartitionTables
	it.UnsupportedPartitioningModes = []disk.PartitioningMode{disk.RawPartitioningMode}

	return it
}

func mkMinimalrawImgType() *rhel.ImageType {
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
		SystemdUnit:     systemdUnits,
		// NOTE: temporary workaround for a bug in initial-setup that
		// requires a kickstart file in the root directory.
		Files: []*fsnode.File{initialSetupKickstart()},
	}
	it.KernelOptions = []string{"ro"}
	it.DefaultSize = 2 * datasizes.GibiByte
	it.Bootable = true
	it.BasePartitionTables = minimalrawPartitionTables

	return it
}

var (
	// Shared Services
	edgeServices = []string{
		// TODO(runcom): move fdo-client-linuxapp.service to presets?
		"NetworkManager.service", "firewalld.service", "sshd.service", "fdo-client-linuxapp.service",
	}
	minimalrawServices = []string{
		"NetworkManager.service", "firewalld.service", "sshd.service", "initial-setup.service",
	}
	//dropin to disable grub-boot-success.timer if greenboot present
	systemdUnits = []*osbuild.SystemdUnitStageOptions{
		{
			Unit:     "grub-boot-success.timer",
			Dropin:   "10-disable-if-greenboot.conf",
			UnitType: osbuild.Global,
			Config: osbuild.SystemdServiceUnitDropin{
				Unit: &osbuild.SystemdUnitSection{
					FileExists: "!/usr/libexec/greenboot/greenboot",
				},
			},
		},
	}
)

// initialSetupKickstart returns the File configuration for a kickstart file
// that's required to enable initial-setup to run on first boot.
func initialSetupKickstart() *fsnode.File {
	file, err := fsnode.NewFile("/root/anaconda-ks.cfg", nil, "root", "root", []byte("# Run initial-setup on first boot\n# Created by osbuild\nfirstboot --reconfig\nlang en_US.UTF-8\n"))
	if err != nil {
		panic(err)
	}
	return file
}

// Partition tables
func minimalrawPartitionTables(t *rhel.ImageType) (disk.PartitionTable, bool) {
	// RHEL >= 9.3 needs to have a bigger /boot, see RHEL-7999
	bootSize := uint64(600) * datasizes.MebiByte
	if common.VersionLessThan(t.Arch().Distro().OsVersion(), "9.3") && t.IsRHEL() {
		bootSize = 500 * datasizes.MebiByte
	}

	switch t.Arch().Name() {
	case arch.ARCH_X86_64.String():
		return disk.PartitionTable{
			UUID:        "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type:        disk.PT_GPT,
			StartOffset: 8 * datasizes.MebiByte,
			Partitions: []disk.Partition{
				{
					Size: 200 * datasizes.MebiByte,
					Type: disk.EFISystemPartitionGUID,
					UUID: disk.EFISystemPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "vfat",
						UUID:         disk.EFIFilesystemUUID,
						Mountpoint:   "/boot/efi",
						Label:        "EFI-SYSTEM",
						FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
						FSTabFreq:    0,
						FSTabPassNo:  2,
					},
				},
				{
					Size: bootSize,
					Type: disk.XBootLDRPartitionGUID,
					UUID: disk.DataPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Mountpoint:   "/boot",
						Label:        "boot",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
				{
					Size: 2 * datasizes.GibiByte,
					Type: disk.FilesystemDataGUID,
					UUID: disk.RootPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Label:        "root",
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		}, true
	case arch.ARCH_AARCH64.String():
		return disk.PartitionTable{
			UUID:        "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type:        disk.PT_GPT,
			StartOffset: 8 * datasizes.MebiByte,
			Partitions: []disk.Partition{
				{
					Size: 200 * datasizes.MebiByte,
					Type: disk.EFISystemPartitionGUID,
					UUID: disk.EFISystemPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "vfat",
						UUID:         disk.EFIFilesystemUUID,
						Mountpoint:   "/boot/efi",
						Label:        "EFI-SYSTEM",
						FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
						FSTabFreq:    0,
						FSTabPassNo:  2,
					},
				},
				{
					Size: bootSize,
					Type: disk.XBootLDRPartitionGUID,
					UUID: disk.DataPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Mountpoint:   "/boot",
						Label:        "boot",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
				{
					Size: 2 * datasizes.GibiByte,
					Type: disk.FilesystemDataGUID,
					UUID: disk.RootPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Label:        "root",
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		}, true
	default:
		return disk.PartitionTable{}, false
	}
}

func edgeBasePartitionTables(t *rhel.ImageType) (disk.PartitionTable, bool) {
	switch t.Arch().Name() {
	case arch.ARCH_X86_64.String():
		return disk.PartitionTable{
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: disk.PT_GPT,
			Partitions: []disk.Partition{
				{
					Size:     1 * datasizes.MebiByte,
					Bootable: true,
					Type:     disk.BIOSBootPartitionGUID,
					UUID:     disk.BIOSBootPartitionUUID,
				},
				{
					Size: 127 * datasizes.MebiByte,
					Type: disk.EFISystemPartitionGUID,
					UUID: disk.EFISystemPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "vfat",
						UUID:         disk.EFIFilesystemUUID,
						Mountpoint:   "/boot/efi",
						Label:        "EFI-SYSTEM",
						FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
						FSTabFreq:    0,
						FSTabPassNo:  2,
					},
				},
				{
					Size: 384 * datasizes.MebiByte,
					Type: disk.XBootLDRPartitionGUID,
					UUID: disk.DataPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Mountpoint:   "/boot",
						Label:        "boot",
						FSTabOptions: "defaults",
						FSTabFreq:    1,
						FSTabPassNo:  1,
					},
				},
				{
					Type: disk.FilesystemDataGUID,
					UUID: disk.RootPartitionUUID,
					Payload: &disk.LUKSContainer{
						Label:      "crypt_root",
						Cipher:     "cipher_null",
						Passphrase: "osbuild",
						PBKDF: disk.Argon2id{
							Memory:      32,
							Iterations:  4,
							Parallelism: 1,
						},
						Clevis: &disk.ClevisBind{
							Pin:              "null",
							Policy:           "{}",
							RemovePassphrase: true,
						},
						Payload: &disk.LVMVolumeGroup{
							Name:        "rootvg",
							Description: "built with lvm2 and osbuild",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Size: 9 * datasizes.GiB, // 9 GiB
									Name: "rootlv",
									Payload: &disk.Filesystem{
										Type:         "xfs",
										Label:        "root",
										Mountpoint:   "/",
										FSTabOptions: "defaults",
										FSTabFreq:    0,
										FSTabPassNo:  0,
									},
								},
							},
						},
					},
				},
			},
		}, true
	case arch.ARCH_AARCH64.String():
		return disk.PartitionTable{
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: disk.PT_GPT,
			Partitions: []disk.Partition{
				{
					Size: 127 * datasizes.MebiByte,
					Type: disk.EFISystemPartitionGUID,
					UUID: disk.EFISystemPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "vfat",
						UUID:         disk.EFIFilesystemUUID,
						Mountpoint:   "/boot/efi",
						Label:        "EFI-SYSTEM",
						FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
						FSTabFreq:    0,
						FSTabPassNo:  2,
					},
				},
				{
					Size: 384 * datasizes.MebiByte,
					Type: disk.XBootLDRPartitionGUID,
					UUID: disk.DataPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Mountpoint:   "/boot",
						Label:        "boot",
						FSTabOptions: "defaults",
						FSTabFreq:    1,
						FSTabPassNo:  1,
					},
				},
				{
					Type: disk.FilesystemDataGUID,
					UUID: disk.RootPartitionUUID,
					Payload: &disk.LUKSContainer{
						Label:      "crypt_root",
						Cipher:     "cipher_null",
						Passphrase: "osbuild",
						PBKDF: disk.Argon2id{
							Memory:      32,
							Iterations:  4,
							Parallelism: 1,
						},
						Clevis: &disk.ClevisBind{
							Pin:              "null",
							Policy:           "{}",
							RemovePassphrase: true,
						},
						Payload: &disk.LVMVolumeGroup{
							Name:        "rootvg",
							Description: "built with lvm2 and osbuild",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Size: 9 * datasizes.GiB, // 9 GiB
									Name: "rootlv",
									Payload: &disk.Filesystem{
										Type:         "xfs",
										Label:        "root",
										Mountpoint:   "/",
										FSTabOptions: "defaults",
										FSTabFreq:    0,
										FSTabPassNo:  0,
									},
								},
							},
						},
					},
				},
			},
		}, true

	default:
		return disk.PartitionTable{}, false
	}
}

// Package Sets

// edge commit OS package set
func edgeCommitPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"redhat-release",
			"glibc",
			"glibc-minimal-langpack",
			"nss-altfiles",
			"dracut-config-generic",
			"dracut-network",
			"basesystem",
			"bash",
			"platform-python",
			"shadow-utils",
			"chrony",
			"setup",
			"shadow-utils",
			"sudo",
			"systemd",
			"coreutils",
			"util-linux",
			"curl",
			"vim-minimal",
			"rpm",
			"rpm-ostree",
			"polkit",
			"lvm2",
			"cryptsetup",
			"pinentry",
			"e2fsprogs",
			"dosfstools",
			"keyutils",
			"gnupg2",
			"attr",
			"xz",
			"gzip",
			"firewalld",
			"iptables",
			"NetworkManager",
			"NetworkManager-wifi",
			"NetworkManager-wwan",
			"wpa_supplicant",
			"traceroute",
			"hostname",
			"iproute",
			"iputils",
			"openssh-clients",
			"procps-ng",
			"rootfiles",
			"openssh-server",
			"passwd",
			"policycoreutils",
			"policycoreutils-python-utils",
			"selinux-policy-targeted",
			"setools-console",
			"less",
			"tar",
			"rsync",
			"usbguard",
			"bash-completion",
			"tmux",
			"ima-evm-utils",
			"audit",
			"podman",
			"containernetworking-plugins", // required for cni networks but not a hard dependency of podman >= 4.2.0 (rhbz#2123210)
			"container-selinux",
			"skopeo",
			"criu",
			"slirp4netns",
			"fuse-overlayfs",
			"clevis",
			"clevis-dracut",
			"clevis-luks",
			"greenboot",
			"greenboot-default-health-checks",
			"fdo-client",
			"fdo-owner-cli",
			"sos",
		},
		Exclude: []string{
			"rng-tools",
			"bootupd",
		},
	}

	switch t.Arch().Name() {
	case arch.ARCH_X86_64.String():
		ps = ps.Append(x8664EdgeCommitPackageSet(t))

	case arch.ARCH_AARCH64.String():
		ps = ps.Append(aarch64EdgeCommitPackageSet(t))
	}

	if common.VersionGreaterThanOrEqual(t.Arch().Distro().OsVersion(), "9.2") || !t.IsRHEL() {
		ps.Include = append(ps.Include, "ignition", "ignition-edge", "ssh-key-dir")
	}

	if common.VersionLessThan(t.Arch().Distro().OsVersion(), "9.6") {
		// dnsmasq removed in 9.6+ but kept in older versions
		ps.Include = append(ps.Include, "dnsmasq")
	}

	return ps

}

func x8664EdgeCommitPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
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

func aarch64EdgeCommitPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"grub2-efi-aa64",
			"efibootmgr",
			"shim-aa64",
			"iwl7260-firmware",
		},
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
