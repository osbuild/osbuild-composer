package rhel9

import (
	"fmt"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

var (
	// Image Definitions
	edgeCommitImgType = imageType{
		name:        "edge-commit",
		nameAliases: []string{"rhel-edge-commit"},
		filename:    "commit.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: edgeBuildPackageSet,
			osPkgsKey:    edgeCommitPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: edgeServices,
		},
		rpmOstree:        true,
		pipelines:        edgeCommitPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "commit-archive"},
		exports:          []string{"commit-archive"},
	}

	edgeOCIImgType = imageType{
		name:        "edge-container",
		nameAliases: []string{"rhel-edge-container"},
		filename:    "container.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: edgeBuildPackageSet,
			osPkgsKey:    edgeCommitPackageSet,
			containerPkgsKey: func(t *imageType) rpmmd.PackageSet {
				return rpmmd.PackageSet{
					Include: []string{"nginx"},
				}
			},
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: edgeServices,
		},
		rpmOstree:        true,
		bootISO:          false,
		pipelines:        edgeContainerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "container-tree", "container"},
		exports:          []string{containerPkgsKey},
	}

	edgeRawImgType = imageType{
		name:        "edge-raw-image",
		nameAliases: []string{"rhel-edge-raw-image"},
		filename:    "image.raw.xz",
		mimeType:    "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: edgeRawImageBuildPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: common.StringToPtr("en_US.UTF-8"),
		},
		defaultSize:         10 * common.GibiByte,
		rpmOstree:           true,
		bootable:            true,
		bootISO:             false,
		pipelines:           edgeRawImagePipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"image-tree", "image", "archive"},
		exports:             []string{"archive"},
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
			buildPkgsKey:     edgeInstallerBuildPackageSet,
			osPkgsKey:        edgeCommitPackageSet,
			installerPkgsKey: edgeInstallerPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale:          common.StringToPtr("en_US.UTF-8"),
			EnabledServices: edgeServices,
		},
		rpmOstree:        true,
		bootISO:          true,
		pipelines:        edgeInstallerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "bootiso-tree", "bootiso"},
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
			buildPkgsKey:     edgeSimplifiedInstallerBuildPackageSet,
			installerPkgsKey: edgeSimplifiedInstallerPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: edgeServices,
		},
		defaultSize:         10 * common.GibiByte,
		rpmOstree:           true,
		bootable:            true,
		bootISO:             true,
		pipelines:           edgeSimplifiedInstallerPipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"image-tree", "image", "archive", "coi-tree", "efiboot-tree", "bootiso-tree", "bootiso"},
		exports:             []string{"bootiso"},
		basePartitionTables: edgeBasePartitionTables,
	}

	// Shared Services
	edgeServices = []string{
		// TODO(runcom): move fdo-client-linuxapp.service to presets?
		"NetworkManager.service", "firewalld.service", "sshd.service", "fdo-client-linuxapp.service",
	}

	// Partition tables
	edgeBasePartitionTables = distro.BasePartitionTableMap{
		distro.X86_64ArchName: disk.PartitionTable{
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: "gpt",
			Partitions: []disk.Partition{
				{
					Size:     1 * common.MebiByte, // 1MB
					Bootable: true,
					Type:     disk.BIOSBootPartitionGUID,
					UUID:     disk.BIOSBootPartitionUUID,
				},
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
		},
		distro.Aarch64ArchName: disk.PartitionTable{
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
		},
	}
)

// Package Sets

// common edge image build package set
func edgeBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return distroBuildPackageSet(t).Append(
		rpmmd.PackageSet{
			Include: []string{
				"rpm-ostree",
			},
		})
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

func edgeSimplifiedInstallerBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return edgeInstallerBuildPackageSet(t).Append(
		edgeEncryptionBuildPackageSet(t),
	)
}

func edgeRawImageBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return edgeBuildPackageSet(t).Append(edgeEncryptionBuildPackageSet(t)).Append(
		bootPackageSet(t),
	)
}
func edgeInstallerBuildPackageSet(t *imageType) rpmmd.PackageSet {
	return anacondaBuildPackageSet(t).Append(
		edgeBuildPackageSet(t),
	)
}

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
		},
		Exclude: []string{
			"rng-tools",
		},
	}

	ps = ps.Append(bootPackageSet(t))

	switch t.arch.Name() {
	case distro.X86_64ArchName:
		ps = ps.Append(x8664EdgeCommitPackageSet(t))

	case distro.Aarch64ArchName:
		ps = ps.Append(aarch64EdgeCommitPackageSet(t))
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
			"rootfiles",
			"setools-console",
			"sudo",
			"traceroute",
			"util-linux",
		},
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
