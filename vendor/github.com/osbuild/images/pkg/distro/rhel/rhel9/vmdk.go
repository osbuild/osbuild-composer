package rhel9

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/rpmmd"
)

func vmdkKernelOptions() []string {
	return []string{"ro", "net.ifnames=0"}
}

func mkVMDKImgType() *rhel.ImageType {
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

	it.DefaultImageConfig = &distro.ImageConfig{
		Locale: common.ToPtr("en_US.UTF-8"),
	}
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
			rhel.OSPkgsKey: vmdkCommonPackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vmdk", "ovf", "archive"},
		[]string{"archive"},
	)

	it.DefaultImageConfig = &distro.ImageConfig{
		Locale: common.ToPtr("en_US.UTF-8"),
	}
	it.KernelOptions = vmdkKernelOptions()
	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func vmdkCommonPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"@core",
			"chrony",
			"cloud-init",
			"firewalld",
			"langpacks-en",
			"open-vm-tools",
			"tuned",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"rng-tools",
		},
	}

	return ps
}
