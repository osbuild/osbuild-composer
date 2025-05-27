package rhel10

import (
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkVMDKImgType(d *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"vmdk",
		"disk.vmdk",
		"application/x-vmdk",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vmdk"},
		[]string{"vmdk"},
	)
	it.DefaultImageConfig = imageConfig(d, "x86_64", "vmdk")
	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkOVAImgType(d *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"ova",
		"image.ova",
		"application/ovf",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vmdk", "ovf", "archive"},
		[]string{"archive"},
	)
	it.DefaultImageConfig = imageConfig(d, "x86_64", "ova")
	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}
