package rhel8

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkGceImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"gce",
		"image.tar.gz",
		"application/gzip",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "archive"},
		[]string{"archive"},
	)

	it.DefaultImageConfig = imageConfig(rd, a.String(), "gce")
	it.Bootable = true
	it.DefaultSize = 20 * datasizes.GibiByte
	// TODO: the base partition table still contains the BIOS boot partition, but the image is UEFI-only
	it.BasePartitionTables = partitionTables

	return it
}

func mkGceRhuiImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"gce-rhui",
		"image.tar.gz",
		"application/gzip",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "archive"},
		[]string{"archive"},
	)

	it.DefaultImageConfig = imageConfig(rd, a.String(), "gce-rhui")
	it.Bootable = true
	it.DefaultSize = 20 * datasizes.GibiByte
	// TODO: the base partition table still contains the BIOS boot partition, but the image is UEFI-only
	it.BasePartitionTables = partitionTables

	return it
}
