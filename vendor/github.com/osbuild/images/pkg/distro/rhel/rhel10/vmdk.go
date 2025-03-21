package rhel10

import (
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func vmdkKernelOptions() []string {
	return []string{"ro"}
}

func mkVMDKImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"vmdk",
		"disk.vmdk",
		"application/x-vmdk",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vmdk"},
		[]string{"vmdk"},
	)

	it.KernelOptions = vmdkKernelOptions()
	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkOVAImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"ova",
		"image.ova",
		"application/ovf",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vmdk", "ovf", "archive"},
		[]string{"archive"},
	)

	it.KernelOptions = vmdkKernelOptions()
	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}
