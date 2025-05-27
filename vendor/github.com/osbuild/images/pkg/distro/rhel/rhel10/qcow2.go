package rhel10

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkQcow2ImgType(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"qcow2",
		"disk.qcow2",
		"application/x-qemu-disk",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "qcow2"},
		[]string{"qcow2"},
	)

	it.DefaultImageConfig = imageConfig(d, a.String(), "qcow2")
	it.DefaultSize = 10 * datasizes.GibiByte
	it.Bootable = true
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkOCIImgType(d *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"oci",
		"disk.qcow2",
		"application/x-qemu-disk",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "qcow2"},
		[]string{"qcow2"},
	)

	it.DefaultImageConfig = imageConfig(d, "", "oci")
	it.DefaultSize = 10 * datasizes.GibiByte
	it.Bootable = true
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}
