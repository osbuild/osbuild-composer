package rhel9

import (
	"fmt"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/fsnode"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

var (
	// Image Definitions
	edgeCommitImgType = imageType{
		name:        "edge-commit",
		nameAliases: []string{"rhel-edge-commit"},
		filename:    "commit.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: edgeCommitPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: edgeServices,
			SystemdUnit:     systemdUnits,
		},
		rpmOstree:        true,
		image:            edgeCommitImage,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "ostree-commit", "commit-archive"},
		exports:          []string{"commit-archive"},
	}

	edgeOCIImgType = imageType{
		name:        "edge-container",
		nameAliases: []string{"rhel-edge-container"},
		filename:    "container.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: edgeCommitPackageSet,
			containerPkgsKey: func(t *imageType) rpmmd.PackageSet {
				return rpmmd.PackageSet{
					Include: []string{"nginx"}, // FIXME: this has no effect
				}
			},
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: edgeServices,
			SystemdUnit:     systemdUnits,
		},
		rpmOstree:        true,
		bootISO:          false,
		image:            edgeContainerImage,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "ostree-commit", "container-tree", "container"},
		exports:          []string{"container"},
	}

	edgeRawImgType = imageType{
		name:        "edge-raw-image",
		nameAliases: []string{"rhel-edge-raw-image"},
		filename:    "image.raw.xz",
		compression: "xz",
		mimeType:    "application/xz",
		packageSets: nil,
		defaultImageConfig: &distro.ImageConfig{
			Locale: common.ToPtr("en_US.UTF-8"),
		},
		defaultSize:         10 * common.GibiByte,
		rpmOstree:           true,
		bootable:            true,
		bootISO:             false,
		image:               edgeRawImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"ostree-deployment", "image", "xz"},
		exports:             []string{"xz"},
		basePartitionTables: edgeBasePartitionTables,
	}

	edgeInstallerImgType = imageType{
		name:        "edge-installer",
		nameAliases: []string{"rhel-edge-installer"},
		filename:    "installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			// TODO: non-arch-specific package set handling for installers
			// This image type requires build packages for installers and
			// ostree/edge.  For now we only have x86-64 installer build
			// package sets defined.  When we add installer build package sets
			// for other architectures, this will need to be moved to the
			// architecture and the merging will happen in the PackageSets()
			// method like the other sets.
			installerPkgsKey: edgeInstallerPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale:          common.ToPtr("en_US.UTF-8"),
			EnabledServices: edgeServices,
		},
		rpmOstree:        true,
		bootISO:          true,
		image:            edgeInstallerImage,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "rootfs-image", "efiboot-tree", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}

	edgeSimplifiedInstallerImgType = imageType{
		name:        "edge-simplified-installer",
		nameAliases: []string{"rhel-edge-simplified-installer"},
		filename:    "simplified-installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			// TODO: non-arch-specific package set handling for installers
			// This image type requires build packages for installers and
			// ostree/edge.  For now we only have x86-64 installer build
			// package sets defined.  When we add installer build package sets
			// for other architectures, this will need to be moved to the
			// architecture and the merging will happen in the PackageSets()
			// method like the other sets.
			installerPkgsKey: edgeSimplifiedInstallerPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: edgeServices,
		},
		defaultSize:         10 * common.GibiByte,
		rpmOstree:           true,
		bootable:            true,
		bootISO:             true,
		image:               edgeSimplifiedInstallerImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"ostree-deployment", "image", "xz", "coi-tree", "efiboot-tree", "bootiso-tree", "bootiso"},
		exports:             []string{"bootiso"},
		basePartitionTables: edgeBasePartitionTables,
	}

	edgeAMIImgType = imageType{
		name:        "edge-ami",
		filename:    "image.raw",
		mimeType:    "application/octet-stream",
		packageSets: nil,

		defaultImageConfig: &distro.ImageConfig{
			Locale: common.ToPtr("en_US.UTF-8"),
		},
		defaultSize:         10 * common.GibiByte,
		rpmOstree:           true,
		bootable:            true,
		bootISO:             false,
		image:               edgeRawImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"ostree-deployment", "image"},
		exports:             []string{"image"},
		basePartitionTables: edgeBasePartitionTables,
		environment:         &environment.EC2{},
	}

	edgeVsphereImgType = imageType{
		name:        "edge-vsphere",
		filename:    "image.vmdk",
		mimeType:    "application/x-vmdk",
		packageSets: nil,
		defaultImageConfig: &distro.ImageConfig{
			Locale: common.ToPtr("en_US.UTF-8"),
		},
		defaultSize:         10 * common.GibiByte,
		rpmOstree:           true,
		bootable:            true,
		bootISO:             false,
		image:               edgeRawImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"ostree-deployment", "image", "vmdk"},
		exports:             []string{"vmdk"},
		basePartitionTables: edgeBasePartitionTables,
	}

	minimalrawImgType = imageType{
		name:        "minimal-raw",
		filename:    "raw.img.xz",
		compression: "xz",
		mimeType:    "application/xz",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: minimalrpmPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: minimalrawServices,
			SystemdUnit:     systemdUnits,
			// NOTE: temporary workaround for a bug in initial-setup that
			// requires a kickstart file in the root directory.
			Files: []*fsnode.File{initialSetupKickstart()},
		},
		rpmOstree:           false,
		kernelOptions:       "ro",
		bootable:            true,
		defaultSize:         2 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "xz"},
		exports:             []string{"xz"},
		basePartitionTables: minimalrawPartitionTables,
	}

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

// Partition tables
func minimalrawPartitionTables(t *imageType) (disk.PartitionTable, bool) {
	// RHEL >= 9.3 needs to have a bigger /boot, see RHEL-7999
	bootSize := uint64(600) * common.MebiByte
	if common.VersionLessThan(t.arch.distro.osVersion, "9.3") && t.arch.distro.isRHEL() {
		bootSize = 500 * common.MebiByte
	}

	switch t.platform.GetArch() {
	case platform.ARCH_X86_64:
		return disk.PartitionTable{
			UUID:        "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type:        "gpt",
			StartOffset: 8 * common.MebiByte,
			Partitions: []disk.Partition{
				{
					Size: 200 * common.MebiByte,
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
					UUID: disk.FilesystemDataUUID,
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
					Size: 2 * common.GibiByte,
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
	case platform.ARCH_AARCH64:
		return disk.PartitionTable{
			UUID:        "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type:        "gpt",
			StartOffset: 8 * common.MebiByte,
			Partitions: []disk.Partition{
				{
					Size: 200 * common.MebiByte,
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
					UUID: disk.FilesystemDataUUID,
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
					Size: 2 * common.GibiByte,
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

func edgeBasePartitionTables(t *imageType) (disk.PartitionTable, bool) {
	switch t.platform.GetArch() {
	case platform.ARCH_X86_64:
		return disk.PartitionTable{
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: "gpt",
			Partitions: []disk.Partition{
				{
					Size: 127 * common.MebiByte, // 127 MB
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
					Size: 384 * common.MebiByte, // 384 MB
					Type: disk.XBootLDRPartitionGUID,
					UUID: disk.FilesystemDataUUID,
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
									Size: 9 * 1024 * 1024 * 1024, // 9 GB
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
	case platform.ARCH_AARCH64:
		return disk.PartitionTable{
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: "gpt",
			Partitions: []disk.Partition{
				{
					Size: 127 * common.MebiByte, // 127 MB
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
					Size: 384 * common.MebiByte, // 384 MB
					Type: disk.XBootLDRPartitionGUID,
					UUID: disk.FilesystemDataUUID,
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
									Size: 9 * 1024 * 1024 * 1024, // 9 GB
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
func edgeCommitPackageSet(t *imageType) rpmmd.PackageSet {
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
			"dnsmasq",
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
		},
	}

	switch t.arch.Name() {
	case platform.ARCH_X86_64.String():
		ps = ps.Append(x8664EdgeCommitPackageSet(t))

	case platform.ARCH_AARCH64.String():
		ps = ps.Append(aarch64EdgeCommitPackageSet(t))
	}

	if !common.VersionLessThan(t.arch.distro.osVersion, "9.2") || !common.VersionLessThan(t.arch.distro.osVersion, "9-stream") {
		ps.Include = append(ps.Include, "ignition", "ignition-edge", "ssh-key-dir")
	}

	return ps

}

func x8664EdgeCommitPackageSet(t *imageType) rpmmd.PackageSet {
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

func aarch64EdgeCommitPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"grub2-efi-aa64",
			"efibootmgr",
			"shim-aa64",
			"iwl7260-firmware",
		},
	}
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
			"redhat-logos",
			"rootfiles",
			"setools-console",
			"sudo",
			"traceroute",
			"util-linux",
		},
	})

	switch t.arch.Name() {

	case platform.ARCH_X86_64.String():
		ps = ps.Append(x8664EdgeCommitPackageSet(t))
	case platform.ARCH_AARCH64.String():
		ps = ps.Append(aarch64EdgeCommitPackageSet(t))

	default:
		panic(fmt.Sprintf("unsupported arch: %s", t.arch.Name()))
	}

	return ps
}
