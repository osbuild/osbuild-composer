package rhel10

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
)

// Azure image type
func mkAzureImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"vhd",
		"disk.vhd",
		"application/x-vhd",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vpc"},
		[]string{"vpc"},
	)

	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.DefaultImageConfig = imageConfig(rd, a.String(), "vhd")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// Azure RHEL-internal image type
func mkAzureInternalImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"azure-rhui",
		"disk.vhd.xz",
		"application/xz",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vpc", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.DefaultImageConfig = imageConfig(rd, a.String(), "azure-rhui")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkAzureSapInternalImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"azure-sap-rhui",
		"disk.vhd.xz",
		"application/xz",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vpc", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.DefaultImageConfig = imageConfig(rd, a.String(), "azure-sap-rhui")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}
