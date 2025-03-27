package rhel8

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
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
			rhel.OSPkgsKey: packageSetLoader,
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
			rhel.OSPkgsKey: packageSetLoader,
			rhel.ContainerPkgsKey: func(t *rhel.ImageType) (rpmmd.PackageSet, error) {
				return defs.PackageSet(t, "edge_container_pipeline_pkgset", nil)
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
	it.KernelOptions = []string{"modprobe.blacklist=vc4"}
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
			rhel.InstallerPkgsKey: packageSetLoader,
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
			rhel.InstallerPkgsKey: packageSetLoader,
		},
		rhel.EdgeSimplifiedInstallerImage,
		[]string{"build"},
		[]string{"ostree-deployment", "image", "xz", "coi-tree", "efiboot-tree", "bootiso-tree", "bootiso"},
		[]string{"bootiso"},
	)

	it.NameAliases = []string{"rhel-edge-simplified-installer"}
	it.KernelOptions = []string{"modprobe.blacklist=vc4"}
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
			rhel.OSPkgsKey: packageSetLoader,
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
	it.KernelOptions = []string{"ro"}
	it.Bootable = true
	it.DefaultSize = 2 * datasizes.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
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
