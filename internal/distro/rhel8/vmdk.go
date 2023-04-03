package rhel8

import (
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const vmdkKernelOptions = "ro net.ifnames=0"

func vmdkImgType() imageType {
	return imageType{
		name:     "vmdk",
		filename: "disk.vmdk",
		mimeType: "application/x-vmdk",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: vmdkCommonPackageSet,
		},
		kernelOptions:       vmdkKernelOptions,
		bootable:            true,
		defaultSize:         4 * common.GibiByte,
		image:               liveImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vmdk"},
		exports:             []string{"vmdk"},
		basePartitionTables: defaultBasePartitionTables,
	}
}

func ovaImgType() imageType {
	return imageType{
		name:     "ova",
		filename: "image.ova",
		mimeType: "application/ovf",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: vmdkCommonPackageSet,
		},
		kernelOptions:       vmdkKernelOptions,
		bootable:            true,
		defaultSize:         4 * common.GibiByte,
		image:               liveImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vmdk", "ovf", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}
}

func vmdkCommonPackageSet(t *imageType) rpmmd.PackageSet {
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
