package rhel8

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkEdgeCommitImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-commit",
		"commit.tar",
		"application/x-tar",
		packageSetLoader,
		rhel.EdgeCommitImage,
		[]string{"build"},
		[]string{"os", "ostree-commit", "commit-archive"},
		[]string{"commit-archive"},
	)

	it.NameAliases = []string{"rhel-edge-commit"}
	it.DefaultImageConfig = imageConfig(rd, a.String(), "edge-commit")
	it.RPMOSTree = true

	return it
}

func mkEdgeOCIImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-container",
		"container.tar",
		"application/x-tar",
		packageSetLoader,
		rhel.EdgeContainerImage,
		[]string{"build"},
		[]string{"os", "ostree-commit", "container-tree", "container"},
		[]string{"container"},
	)

	it.NameAliases = []string{"rhel-edge-container"}
	it.DefaultImageConfig = imageConfig(rd, a.String(), "edge-container")
	it.RPMOSTree = true

	return it
}

func mkEdgeRawImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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
	it.DefaultImageConfig = imageConfig(rd, a.String(), "edge-raw-image")
	it.DefaultSize = 10 * datasizes.GibiByte
	it.RPMOSTree = true
	it.Bootable = true
	it.BasePartitionTables = partitionTables
	it.UnsupportedPartitioningModes = []disk.PartitioningMode{
		disk.AutoLVMPartitioningMode,
		disk.LVMPartitioningMode,
	}

	return it
}

func mkEdgeInstallerImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-installer",
		"installer.iso",
		"application/x-iso9660-image",
		packageSetLoader,
		rhel.EdgeInstallerImage,
		[]string{"build"},
		[]string{"anaconda-tree", "rootfs-image", "efiboot-tree", "bootiso-tree", "bootiso"},
		[]string{"bootiso"},
	)

	it.NameAliases = []string{"rhel-edge-installer"}
	it.DefaultImageConfig = imageConfig(rd, a.String(), "edge-installer")
	it.DefaultInstallerConfig = &distro.InstallerConfig{
		AdditionalDracutModules: []string{
			"ifcfg",
		},
	}
	it.RPMOSTree = true
	it.BootISO = true
	it.ISOLabelFn = distroISOLabelFunc

	return it
}

func mkEdgeSimplifiedInstallerImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-simplified-installer",
		"simplified-installer.iso",
		"application/x-iso9660-image",
		packageSetLoader,
		rhel.EdgeSimplifiedInstallerImage,
		[]string{"build"},
		[]string{"ostree-deployment", "image", "xz", "coi-tree", "efiboot-tree", "bootiso-tree", "bootiso"},
		[]string{"bootiso"},
	)

	it.NameAliases = []string{"rhel-edge-simplified-installer"}
	it.DefaultImageConfig = imageConfig(rd, a.String(), "edge-simplified-installer")
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
	it.BasePartitionTables = partitionTables
	it.UnsupportedPartitioningModes = []disk.PartitioningMode{
		disk.AutoLVMPartitioningMode,
		disk.LVMPartitioningMode,
	}

	return it
}

func mkMinimalRawImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"minimal-raw",
		"disk.raw.xz",
		"application/xz",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.DefaultImageConfig = imageConfig(rd, a.String(), "minimal-raw")
	it.Bootable = true
	it.DefaultSize = 2 * datasizes.GibiByte
	it.BasePartitionTables = partitionTables

	return it
}
