package rhel8

import (
	"fmt"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func edgeCommitImgType(rd distribution) imageType {
	it := imageType{
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
			EnabledServices: edgeServices(rd),
		},
		rpmOstree:        true,
		pipelines:        edgeCommitPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "commit-archive"},
		exports:          []string{"commit-archive"},
	}
	return it
}

func edgeOCIImgType(rd distribution) imageType {
	it := imageType{
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
			EnabledServices: edgeServices(rd),
		},
		rpmOstree:        true,
		bootISO:          false,
		pipelines:        edgeContainerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "container-tree", "container"},
		exports:          []string{"container"},
	}
	return it
}
func edgeRawImgType() imageType {
	it := imageType{
		name:        "edge-raw-image",
		nameAliases: []string{"rhel-edge-raw-image"},
		filename:    "image.raw.xz",
		mimeType:    "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: edgeRawImageBuildPackageSet,
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
	return it
}

func edgeInstallerImgType(rd distribution) imageType {
	it := imageType{
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
			EnabledServices: edgeServices(rd),
		},
		rpmOstree:        true,
		bootISO:          true,
		pipelines:        edgeInstallerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}
	return it
}

func edgeSimplifiedInstallerImgType(rd distribution) imageType {
	it := imageType{
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
			EnabledServices: edgeServices(rd),
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
	return it
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

	if t.arch.distro.isRHEL() && common.VersionLessThan(t.arch.distro.osVersion, "8.6") {
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

func edgeServices(rd distribution) []string {
	// Common Services
	var edgeServices = []string{"NetworkManager.service", "firewalld.service", "sshd.service"}

	if rd.osVersion == "8.4" {
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

	if !(rd.isRHEL() && common.VersionLessThan(rd.osVersion, "8.6")) {
		// enable fdo-client only on RHEL 8.6+ and CS8

		// TODO(runcom): move fdo-client-linuxapp.service to presets?
		edgeServices = append(edgeServices, "fdo-client-linuxapp.service")
	}

	return edgeServices
}
