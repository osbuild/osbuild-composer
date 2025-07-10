package rhel9

import (
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkEdgeCommitImgType(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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
	it.RPMOSTree = true
	it.DefaultImageConfig = imageConfig(d, a.String(), "edge-commit")

	return it
}

func mkEdgeOCIImgType(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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
	it.RPMOSTree = true
	it.DefaultImageConfig = imageConfig(d, a.String(), "edge-container")

	return it
}

func mkEdgeRawImgType(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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
	it.DefaultImageConfig = imageConfig(d, a.String(), "edge-raw-image")

	it.DefaultSize = 10 * datasizes.GibiByte
	it.RPMOSTree = true
	it.Bootable = true
	it.BasePartitionTables = defaultBasePartitionTables
	it.UnsupportedPartitioningModes = []disk.PartitioningMode{disk.RawPartitioningMode}

	return it
}

func mkEdgeInstallerImgType(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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
	it.DefaultImageConfig = imageConfig(d, a.String(), "edge-installer")
	it.DefaultInstallerConfig = &distro.InstallerConfig{
		AdditionalDracutModules: []string{
			"nvdimm", // non-volatile DIMM firmware (provides nfit, cuse, and nd_e820)
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

func mkEdgeSimplifiedInstallerImgType(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"edge-simplified-installer",
		"simplified-installer.iso",
		"application/x-iso9660-image",
		// TODO: non-arch-specific package set handling for installers
		// This image type requires build packages for installers and
		// ostree/edge.  For now we only have x86-64 installer build
		// package sets defined.  When we add installer build package sets
		// for other architectures, this will need to be moved to the
		// architecture and the merging will happen in the PackageSets()
		// method like the other sets.
		packageSetLoader,
		rhel.EdgeSimplifiedInstallerImage,
		[]string{"build"},
		[]string{"ostree-deployment", "image", "xz", "coi-tree", "efiboot-tree", "bootiso-tree", "bootiso"},
		[]string{"bootiso"},
	)

	it.NameAliases = []string{"rhel-edge-simplified-installer"}
	it.DefaultImageConfig = imageConfig(d, a.String(), "edge-simplified-installer")

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
	it.BasePartitionTables = defaultBasePartitionTables
	it.UnsupportedPartitioningModes = []disk.PartitioningMode{disk.RawPartitioningMode}

	return it
}

func mkEdgeAMIImgType(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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

	it.DefaultImageConfig = imageConfig(d, a.String(), "edge-ami")

	it.DefaultSize = 10 * datasizes.GibiByte
	it.RPMOSTree = true
	it.Bootable = true
	it.BasePartitionTables = defaultBasePartitionTables
	it.UnsupportedPartitioningModes = []disk.PartitioningMode{disk.RawPartitioningMode}
	it.Environment = &environment.EC2{}

	return it
}

func mkEdgeVsphereImgType(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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

	it.DefaultImageConfig = imageConfig(d, a.String(), "edge-vsphere")
	it.DefaultSize = 10 * datasizes.GibiByte
	it.RPMOSTree = true
	it.Bootable = true
	it.BasePartitionTables = defaultBasePartitionTables
	it.UnsupportedPartitioningModes = []disk.PartitioningMode{disk.RawPartitioningMode}

	return it
}

func mkMinimalrawImgType(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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
	it.DefaultImageConfig = imageConfig(d, a.String(), "minimal-raw")
	it.DefaultSize = 2 * datasizes.GibiByte
	it.Bootable = true
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}
