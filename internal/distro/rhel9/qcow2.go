package rhel9

import (
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

var (
	qcow2ImgType = imageType{
		name:          "qcow2",
		filename:      "disk.qcow2",
		mimeType:      "application/x-qemu-disk",
		kernelOptions: "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: qcow2CommonPackageSet,
		},
		bootable:            true,
		defaultSize:         10 * common.GibiByte,
		image:               liveImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}

	openstackImgType = imageType{
		name:     "openstack",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: openstackCommonPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: common.StringToPtr("en_US.UTF-8"),
		},
		kernelOptions:       "ro net.ifnames=0",
		bootable:            true,
		defaultSize:         4 * common.GibiByte,
		image:               liveImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}
)

func qcow2CommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"authselect-compat",
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
			"tcpdump",
		},
		Exclude: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-lib",
			"alsa-tools-firmware",
			"biosdevname",
			"dnf-plugin-spacewalk",
			"fedora-release",
			"fedora-repos",
			"iprutils",
			"ivtv-firmware",
			"langpacks-*",
			"langpacks-en",
			"libertas-sd8787-firmware",
			"nss",
			"plymouth",
			"rng-tools",
			"udisks2",
		},
	}.Append(bootPackageSet(t)).Append(coreOsCommonPackageSet(t)).Append(distroSpecificPackageSet(t))

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
			"langpacks-en",
			"firewalld",

			// From the lorax kickstart
			"cloud-init",
			"qemu-guest-agent",
			"spice-vdagent",
		},
		Exclude: []string{
			"rng-tools",
		},
	}.Append(bootPackageSet(t)).Append(coreOsCommonPackageSet(t))

	if t.arch.Name() == distro.X86_64ArchName {
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

func qcowImageConfig(d distribution) *distro.ImageConfig {
	ic := &distro.ImageConfig{
		DefaultTarget: common.StringToPtr("multi-user.target"),
	}
	if d.isRHEL() {
		ic.RHSMConfig = map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
			distro.RHSMConfigNoSubscription: {
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
	it := qcow2ImgType
	it.defaultImageConfig = qcowImageConfig(d)
	return it
}
