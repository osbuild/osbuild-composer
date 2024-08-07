package rhel8

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/rpmmd"
)

func mkQcow2ImgType(rd *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"qcow2",
		"disk.qcow2",
		"application/x-qemu-disk",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: qcow2CommonPackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "qcow2"},
		[]string{"qcow2"},
	)

	it.DefaultImageConfig = qcowImageConfig(rd)
	it.KernelOptions = "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0 crashkernel=auto"
	it.Bootable = true
	it.DefaultSize = 10 * common.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkOCIImgType(rd *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"oci",
		"disk.qcow2",
		"application/x-qemu-disk",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: qcow2CommonPackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "qcow2"},
		[]string{"qcow2"},
	)

	it.DefaultImageConfig = qcowImageConfig(rd)
	it.KernelOptions = "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0 crashkernel=auto"
	it.Bootable = true
	it.DefaultSize = 10 * common.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkOpenstackImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"openstack",
		"disk.qcow2",
		"application/x-qemu-disk",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: openstackCommonPackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "qcow2"},
		[]string{"qcow2"},
	)

	it.KernelOptions = "ro net.ifnames=0"
	it.DefaultSize = 4 * common.GibiByte
	it.Bootable = true
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func qcow2CommonPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"@core",
			"authselect-compat",
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"cockpit-system",
			"cockpit-ws",
			"dhcp-client",
			"dnf",
			"dnf-utils",
			"dosfstools",
			"dracut-norescue",
			"net-tools",
			"NetworkManager",
			"nfs-utils",
			"oddjob",
			"oddjob-mkhomedir",
			"psmisc",
			"python3-jsonschema",
			"qemu-guest-agent",
			"redhat-release",
			"redhat-release-eula",
			"rsync",
			"tar",
			"tcpdump",
			"yum",
		},
		Exclude: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-lib",
			"alsa-tools-firmware",
			"biosdevname",
			"dnf-plugin-spacewalk",
			"dracut-config-rescue",
			"fedora-release",
			"fedora-repos",
			"firewalld",
			"fwupd",
			"iprutils",
			"ivtv-firmware",
			"iwl1000-firmware",
			"iwl100-firmware",
			"iwl105-firmware",
			"iwl135-firmware",
			"iwl2000-firmware",
			"iwl2030-firmware",
			"iwl3160-firmware",
			"iwl3945-firmware",
			"iwl4965-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6000-firmware",
			"iwl6000g2a-firmware",
			"iwl6000g2b-firmware",
			"iwl6050-firmware",
			"iwl7260-firmware",
			"langpacks-*",
			"langpacks-en",
			"langpacks-en",
			"libertas-sd8686-firmware",
			"libertas-sd8787-firmware",
			"libertas-usb8388-firmware",
			"nss",
			"plymouth",
			"rng-tools",
			"udisks2",
		},
	}.Append(distroSpecificPackageSet(t))

	// Ensure to not pull in subscription-manager on non-RHEL distro
	if t.IsRHEL() {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"subscription-manager-cockpit",
			},
		})
	}

	return ps
}

func openstackCommonPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			// Defaults
			"@Core",
			"langpacks-en",

			// From the lorax kickstart
			"selinux-policy-targeted",
			"cloud-init",
			"qemu-guest-agent",
			"spice-vdagent",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"rng-tools",
		},
	}
}

func qcowImageConfig(d *rhel.Distribution) *distro.ImageConfig {
	ic := &distro.ImageConfig{
		DefaultTarget: common.ToPtr("multi-user.target"),
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
