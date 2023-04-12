package rhel7

import (
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/subscription"
)

var qcow2ImgType = imageType{
	name:          "qcow2",
	filename:      "disk.qcow2",
	mimeType:      "application/x-qemu-disk",
	kernelOptions: "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0 crashkernel=auto",
	packageSets: map[string]packageSetFunc{
		osPkgsKey: qcow2CommonPackageSet,
	},
	defaultImageConfig:  qcow2DefaultImgConfig,
	bootable:            true,
	defaultSize:         10 * common.GibiByte,
	image:               liveImage,
	buildPipelines:      []string{"build"},
	payloadPipelines:    []string{"os", "image", "qcow2"},
	exports:             []string{"qcow2"},
	basePartitionTables: defaultBasePartitionTables,
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

func qcow2CommonPackageSet(t *imageType) rpmmd.PackageSet {
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
