package rhel7

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/subscription"
)

func mkQcow2ImgType() *rhel.ImageType {
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

	// all RHEL 7 images should use sgdisk
	it.DiskImagePartTool = common.ToPtr(osbuild.PTSgdisk)

	it.KernelOptions = "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0 crashkernel=auto"
	it.Bootable = true
	it.DefaultSize = 10 * common.GibiByte
	it.DefaultImageConfig = qcow2DefaultImgConfig
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

var qcow2DefaultImgConfig = &distro.ImageConfig{
	DefaultTarget:       common.ToPtr("multi-user.target"),
	SELinuxForceRelabel: common.ToPtr(true),
	Sysconfig: []*osbuild.SysconfigStageOptions{
		{
			Kernel: &osbuild.SysconfigKernelOptions{
				UpdateDefault: true,
				DefaultKernel: "kernel",
			},
			Network: &osbuild.SysconfigNetworkOptions{
				Networking: true,
				NoZeroConf: true,
			},
			NetworkScripts: &osbuild.NetworkScriptsOptions{
				IfcfgFiles: map[string]osbuild.IfcfgFile{
					"eth0": {
						Device:    "eth0",
						Bootproto: osbuild.IfcfgBootprotoDHCP,
						OnBoot:    common.ToPtr(true),
						Type:      osbuild.IfcfgTypeEthernet,
						UserCtl:   common.ToPtr(true),
						PeerDNS:   common.ToPtr(true),
						IPv6Init:  common.ToPtr(false),
					},
				},
			},
		},
	},
	RHSMConfig: map[subscription.RHSMStatus]*osbuild.RHSMStageOptions{
		subscription.RHSMConfigNoSubscription: {
			YumPlugins: &osbuild.RHSMStageOptionsDnfPlugins{
				ProductID: &osbuild.RHSMStageOptionsDnfPlugin{
					Enabled: false,
				},
				SubscriptionManager: &osbuild.RHSMStageOptionsDnfPlugin{
					Enabled: false,
				},
			},
		},
	},
}

func qcow2CommonPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"@core",
			"kernel",
			"nfs-utils",
			"yum-utils",

			"cloud-init",
			//"ovirt-guest-agent-common",
			"rhn-setup",
			"yum-rhn-plugin",
			"cloud-utils-growpart",
			"dracut-config-generic",
			"tar",
			"tcpdump",
			"rsync",
		},
		Exclude: []string{
			"biosdevname",
			"dracut-config-rescue",
			"iprutils",
			"NetworkManager-team",
			"NetworkManager-tui",
			"NetworkManager",
			"plymouth",

			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-lib",
			"alsa-tools-firmware",
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
			"libertas-sd8686-firmware",
			"libertas-sd8787-firmware",
			"libertas-usb8388-firmware",
		},
	}.Append(distroSpecificPackageSet(t))

	return ps
}
