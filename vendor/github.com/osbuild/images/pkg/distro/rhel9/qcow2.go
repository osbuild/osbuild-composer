package rhel9

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/subscription"
)

var (
	openstackImgType = imageType{
		name:     "openstack",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: openstackCommonPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: common.ToPtr("en_US.UTF-8"),
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
)

func qcow2CommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"@core",
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"cockpit-system",
			"cockpit-ws",
			"dnf-utils",
			"dosfstools",
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
			"tuned",
			"tcpdump",
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
			"libertas-sd8787-firmware",
			"nss",
			"plymouth",
			"rng-tools",
			"udisks2",
		},
	}.Append(distroSpecificPackageSet(t))

	if t.arch.distro.releaseVersion == "9" {
		ps.Include = append(ps.Include, "authselect-compat")
	}

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
	ps := rpmmd.PackageSet{
		Include: []string{
			// Defaults
			"@core",
			"langpacks-en",
			"tuned",

			// From the lorax kickstart
			"cloud-init",
			"qemu-guest-agent",
			"spice-vdagent",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"rng-tools",
		},
	}

	return ps
}

func qcowImageConfig(d distribution) *distro.ImageConfig {
	ic := &distro.ImageConfig{
		DefaultTarget: common.ToPtr("multi-user.target"),
	}
	if d.isRHEL() {
		ic.RHSMConfig = map[subscription.RHSMStatus]*osbuild.RHSMStageOptions{
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
	return ic
}

func mkQcow2ImgType(d distribution) imageType {
	it := imageType{
		name:          "qcow2",
		filename:      "disk.qcow2",
		mimeType:      "application/x-qemu-disk",
		kernelOptions: "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: qcow2CommonPackageSet,
		},
		bootable:            true,
		defaultSize:         10 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}
	it.defaultImageConfig = qcowImageConfig(d)
	return it
}
