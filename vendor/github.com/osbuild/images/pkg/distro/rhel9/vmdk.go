package rhel9

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
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
			"chrony",
			"cloud-init",
			"firewalld",
			"langpacks-en",
			"open-vm-tools",
		},
		Exclude: []string{
			"rng-tools",
		},
	}.Append(coreOsCommonPackageSet(t))

	if t.arch.Name() == arch.ARCH_X86_64.String() {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				// packages below used to come from @core group and were not excluded
				// they may not be needed at all, but kept them here to not need
				// to exclude them instead in all other images
				"iwl100-firmware",
				"iwl105-firmware",
				"iwl135-firmware",
				"iwl1000-firmware",
				"iwl2000-firmware",
				"iwl2030-firmware",
				"iwl3160-firmware",
				"iwl5000-firmware",
				"iwl5150-firmware",
				"iwl6000g2a-firmware",
				"iwl6050-firmware",
				"iwl7260-firmware",
			},
		})
	}

	return ps
}
