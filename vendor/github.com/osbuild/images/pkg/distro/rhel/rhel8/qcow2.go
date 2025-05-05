package rhel8

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkQcow2ImgType(rd *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"qcow2",
		"disk.qcow2",
		"application/x-qemu-disk",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "qcow2"},
		[]string{"qcow2"},
	)

	it.DefaultImageConfig = qcowImageConfig(rd)
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.BasePartitionTables = partitionTables

	return it
}

func mkOCIImgType(rd *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"oci",
		"disk.qcow2",
		"application/x-qemu-disk",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "qcow2"},
		[]string{"qcow2"},
	)

	it.DefaultImageConfig = qcowImageConfig(rd)
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.BasePartitionTables = partitionTables

	return it
}

func mkOpenstackImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"openstack",
		"disk.qcow2",
		"application/x-qemu-disk",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "qcow2"},
		[]string{"qcow2"},
	)
	it.DefaultImageConfig = &distro.ImageConfig{
		KernelOptions: []string{"ro", "net.ifnames=0"},
	}
	it.DefaultSize = 4 * datasizes.GibiByte
	it.Bootable = true
	it.BasePartitionTables = partitionTables

	return it
}

func qcowImageConfig(d *rhel.Distribution) *distro.ImageConfig {
	ic := &distro.ImageConfig{
		DefaultTarget: common.ToPtr("multi-user.target"),
		KernelOptions: []string{"console=tty0", "console=ttyS0,115200n8", "no_timer_check", "net.ifnames=0", "crashkernel=auto"},
	}
	if d.IsRHEL() {
		ic.RHSMConfig = map[subscription.RHSMStatus]*subscription.RHSMConfig{
			subscription.RHSMConfigNoSubscription: {
				DnfPlugins: subscription.SubManDNFPluginsConfig{
					ProductID: subscription.DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
					SubscriptionManager: subscription.DNFPluginConfig{
						Enabled: common.ToPtr(false),
					},
				},
			},
		}
	}
	return ic
}
