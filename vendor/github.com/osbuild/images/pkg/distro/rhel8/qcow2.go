package rhel8

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/subscription"
)

func qcow2ImgType(rd distribution) imageType {
	it := imageType{
		name:          "qcow2",
		filename:      "disk.qcow2",
		mimeType:      "application/x-qemu-disk",
		kernelOptions: "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0 crashkernel=auto",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: qcow2CommonPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			DefaultTarget: common.ToPtr("multi-user.target"),
		},
		bootable:            true,
		defaultSize:         10 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}

	if rd.isRHEL() {
		it.defaultImageConfig.RHSMConfig = map[subscription.RHSMStatus]*osbuild.RHSMStageOptions{
			subscription.RHSMConfigNoSubscription: {
				DnfPlugins: &osbuild.RHSMStageOptionsDnfPlugins{
					ProductID: &osbuild.RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
					SubscriptionManager: &osbuild.RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
				},
			},
		}
	}

	return it
}

func openstackImgType() imageType {
	return imageType{
		name:     "openstack",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: openstackCommonPackageSet,
		},
		kernelOptions:       "ro net.ifnames=0",
		bootable:            true,
		defaultSize:         4 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}
}

func qcow2CommonPackageSet(t *imageType) rpmmd.PackageSet {
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
	if t.arch.distro.isRHEL() {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{
				"subscription-manager-cockpit",
			},
		})
	}

	return ps
}

func openstackCommonPackageSet(t *imageType) rpmmd.PackageSet {
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
