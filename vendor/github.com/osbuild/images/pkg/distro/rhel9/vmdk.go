package rhel9

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/rpmmd"
)

const vmdkKernelOptions = "ro net.ifnames=0"

var vmdkImgType = imageType{
	name:     "vmdk",
	filename: "disk.vmdk",
	mimeType: "application/x-vmdk",
	packageSets: map[string]packageSetFunc{
		osPkgsKey: vmdkCommonPackageSet,
	},
	defaultImageConfig: &distro.ImageConfig{
		Locale: common.ToPtr("en_US.UTF-8"),
	},
	kernelOptions:       vmdkKernelOptions,
	bootable:            true,
	defaultSize:         4 * common.GibiByte,
	image:               diskImage,
	buildPipelines:      []string{"build"},
	payloadPipelines:    []string{"os", "image", "vmdk"},
	exports:             []string{"vmdk"},
	basePartitionTables: defaultBasePartitionTables,
}

var ovaImgType = imageType{
	name:     "ova",
	filename: "image.ova",
	mimeType: "application/ovf",
	packageSets: map[string]packageSetFunc{
		osPkgsKey: vmdkCommonPackageSet,
	},
	defaultImageConfig: &distro.ImageConfig{
		Locale: common.ToPtr("en_US.UTF-8"),
	},
	kernelOptions:       vmdkKernelOptions,
	bootable:            true,
	defaultSize:         4 * common.GibiByte,
	image:               diskImage,
	buildPipelines:      []string{"build"},
	payloadPipelines:    []string{"os", "image", "vmdk", "ovf", "archive"},
	exports:             []string{"archive"},
	basePartitionTables: defaultBasePartitionTables,
}

func vmdkCommonPackageSet(t *imageType) rpmmd.PackageSet {
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
