package rhel8

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkQcow2ImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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

	it.DefaultImageConfig = imageConfig(rd, a.String(), "qcow2")
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.BasePartitionTables = partitionTables

	return it
}

func mkOCIImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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

	it.DefaultImageConfig = imageConfig(rd, a.String(), "oci")
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.BasePartitionTables = partitionTables

	return it
}

func mkOpenstackImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"openstack",
		"disk.qcow2",
		"application/x-qemu-disk",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "qcow2"},
		[]string{"qcow2"},
	)
	it.DefaultImageConfig = imageConfig(rd, a.String(), "openstack")
	it.DefaultSize = 4 * datasizes.GibiByte
	it.Bootable = true
	it.BasePartitionTables = partitionTables

	return it
}
