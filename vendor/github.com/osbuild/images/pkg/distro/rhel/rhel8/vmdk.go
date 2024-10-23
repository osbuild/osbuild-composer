package rhel8

import (
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/rpmmd"
)

const vmdkKernelOptions = "ro net.ifnames=0"

func mkVmdkImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"vmdk",
		"disk.vmdk",
		"application/x-vmdk",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: vmdkCommonPackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vmdk"},
		[]string{"vmdk"},
	)

	it.KernelOptions = vmdkKernelOptions
	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkOvaImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"ova",
		"image.ova",
		"application/ovf",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: vmdkCommonPackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vmdk", "ovf", "archive"},
		[]string{"archive"},
	)

	it.KernelOptions = vmdkKernelOptions
	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func vmdkCommonPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core",
			"chrony",
			"cloud-init",
			"firewalld",
			"langpacks-en",
			"open-vm-tools",
			"selinux-policy-targeted",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"rng-tools",
		},
	}
}
