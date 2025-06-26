package rhel8

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkAzureRhuiImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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
	it.DefaultImageConfig = imageConfig(rd, a.String(), "azure-rhui")
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.BasePartitionTables = partitionTables

	return it
}

func mkAzureSapRhuiImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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
	it.DefaultImageConfig = imageConfig(rd, a.String(), "azure-sap-rhui")
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.BasePartitionTables = partitionTables

	return it
}

func mkAzureByosImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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

	it.DefaultImageConfig = imageConfig(rd, a.String(), "vhd")
	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.BasePartitionTables = partitionTables

	return it
}

// Azure non-RHEL image type
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

	it.DefaultImageConfig = imageConfig(rd, a.String(), "vhd")
	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.BasePartitionTables = partitionTables

	return it
}

func mkAzureEap7RhuiImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"azure-eap7-rhui",
		"disk.vhd.xz",
		"application/xz",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vpc", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.DefaultImageConfig = imageConfig(rd, a.String(), "azure-eap7-rhui")
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.BasePartitionTables = partitionTables
	it.Workload = eapWorkload()

	return it
}
