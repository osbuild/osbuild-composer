package rhel8

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
)

const gceKernelOptions = "net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y crashkernel=auto console=ttyS0,38400n8d"

func mkGceImgType(rd distro.Distro) *rhel.ImageType {
	it := rhel.NewImageType(
		"gce",
		"image.tar.gz",
		"application/gzip",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: gcePackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "archive"},
		[]string{"archive"},
	)

	it.DefaultImageConfig = defaultGceByosImageConfig(rd)
	it.KernelOptions = gceKernelOptions
	it.Bootable = true
	it.DefaultSize = 20 * datasizes.GibiByte
	// TODO: the base partition table still contains the BIOS boot partition, but the image is UEFI-only
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkGceRhuiImgType(rd distro.Distro) *rhel.ImageType {
	it := rhel.NewImageType(
		"gce-rhui",
		"image.tar.gz",
		"application/gzip",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: gceRhuiPackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "archive"},
		[]string{"archive"},
	)

	it.DefaultImageConfig = defaultGceRhuiImageConfig(rd)
	it.KernelOptions = gceKernelOptions
	it.Bootable = true
	it.DefaultSize = 20 * datasizes.GibiByte
	// TODO: the base partition table still contains the BIOS boot partition, but the image is UEFI-only
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// The configuration for non-RHUI images does not touch the RHSM configuration at all.
// https://issues.redhat.com/browse/COMPOSER-2157
func defaultGceByosImageConfig(rd distro.Distro) *distro.ImageConfig {
	ic := &distro.ImageConfig{
		Timezone: common.ToPtr("UTC"),
		TimeSynchronization: &osbuild.ChronyStageOptions{
			Servers: []osbuild.ChronyConfigServer{{Hostname: "metadata.google.internal"}},
		},
		Firewall: &osbuild.FirewallStageOptions{
			DefaultZone: "trusted",
		},
		EnabledServices: []string{
			"sshd",
			"rngd",
			"dnf-automatic.timer",
		},
		DisabledServices: []string{
			"sshd-keygen@",
			"reboot.target",
		},
		DefaultTarget: common.ToPtr("multi-user.target"),
		Locale:        common.ToPtr("en_US.UTF-8"),
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
		},
		DNFConfig: []*osbuild.DNFConfigStageOptions{
			{
				Config: &osbuild.DNFConfig{
					Main: &osbuild.DNFConfigMain{
						IPResolve: "4",
					},
				},
			},
		},
		DNFAutomaticConfig: &osbuild.DNFAutomaticConfigStageOptions{
			Config: &osbuild.DNFAutomaticConfig{
				Commands: &osbuild.DNFAutomaticConfigCommands{
					ApplyUpdates: common.ToPtr(true),
					UpgradeType:  osbuild.DNFAutomaticUpgradeTypeSecurity,
				},
			},
		},
		YUMRepos: []*osbuild.YumReposStageOptions{
			{
				Filename: "google-cloud.repo",
				Repos: []osbuild.YumRepository{
					{
						Id:           "google-compute-engine",
						Name:         "Google Compute Engine",
						BaseURLs:     []string{"https://packages.cloud.google.com/yum/repos/google-compute-engine-el8-x86_64-stable"},
						Enabled:      common.ToPtr(true),
						GPGCheck:     common.ToPtr(true),
						RepoGPGCheck: common.ToPtr(false),
						GPGKey: []string{
							"https://packages.cloud.google.com/yum/doc/yum-key.gpg",
							"https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg",
						},
					},
				},
			},
		},
		SshdConfig: &osbuild.SshdConfigStageOptions{
			Config: osbuild.SshdConfigConfig{
				PasswordAuthentication: common.ToPtr(false),
				ClientAliveInterval:    common.ToPtr(420),
				PermitRootLogin:        osbuild.PermitRootLoginValueNo,
			},
		},
		Sysconfig: []*osbuild.SysconfigStageOptions{
			{
				Kernel: &osbuild.SysconfigKernelOptions{
					DefaultKernel: "kernel-core",
					UpdateDefault: true,
				},
			},
		},
		Modprobe: []*osbuild.ModprobeStageOptions{
			{
				Filename: "blacklist-floppy.conf",
				Commands: osbuild.ModprobeConfigCmdList{
					osbuild.NewModprobeConfigCmdBlacklist("floppy"),
				},
			},
		},
		GCPGuestAgentConfig: &osbuild.GcpGuestAgentConfigOptions{
			ConfigScope: osbuild.GcpGuestAgentConfigScopeDistro,
			Config: &osbuild.GcpGuestAgentConfig{
				InstanceSetup: &osbuild.GcpGuestAgentConfigInstanceSetup{
					SetBotoConfig: common.ToPtr(false),
				},
			},
		},
	}
	if rd.OsVersion() == "8.4" {
		// NOTE(akoutsou): these are enabled in the package preset, but for
		// some reason do not get enabled on 8.4.
		// the reason is unknown and deeply mysterious
		ic.EnabledServices = append(ic.EnabledServices,
			"google-oslogin-cache.timer",
			"google-guest-agent.service",
			"google-shutdown-scripts.service",
			"google-startup-scripts.service",
			"google-osconfig-agent.service",
		)
	}

	return ic
}

func defaultGceRhuiImageConfig(rd distro.Distro) *distro.ImageConfig {
	ic := &distro.ImageConfig{
		RHSMConfig: map[subscription.RHSMStatus]*subscription.RHSMConfig{
			subscription.RHSMConfigNoSubscription: {
				SubMan: subscription.SubManConfig{
					Rhsmcertd: subscription.SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(true),
					},
					Rhsm: subscription.SubManRHSMConfig{
						ManageRepos: common.ToPtr(false),
					},
				},
			},
			subscription.RHSMConfigWithSubscription: {
				SubMan: subscription.SubManConfig{
					Rhsmcertd: subscription.SubManRHSMCertdConfig{
						AutoRegistration: common.ToPtr(true),
					},
					// do not disable the redhat.repo management if the user
					// explicitly request the system to be subscribed
				},
			},
		},
	}
	ic = ic.InheritFrom(defaultGceByosImageConfig(rd))
	return ic
}

// common GCE image
func gceCommonPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core",
			"langpacks-en", // not in Google's KS
			"acpid",
			"dhcp-client",
			"dnf-automatic",
			"net-tools",
			//"openssh-server", included in core
			"python3",
			"rng-tools",
			"tar",
			"vim",

			// GCE guest tools
			"google-compute-engine",
			"google-osconfig-agent",
			"gce-disk-expand",

			// Not explicitly included in GCP kickstart, but present on the image
			// for time synchronization
			"chrony",
			"timedatex",
			// EFI
			"grub2-tools-efi",
		},
		Exclude: []string{
			"alsa-utils",
			"b43-fwcutter",
			"dmraid",
			"eject",
			"gpm",
			"irqbalance",
			"microcode_ctl",
			"smartmontools",
			"aic94xx-firmware",
			"atmel-firmware",
			"b43-openfwwf",
			"bfa-firmware",
			"ipw2100-firmware",
			"ipw2200-firmware",
			"ivtv-firmware",
			"iwl100-firmware",
			"iwl1000-firmware",
			"iwl3945-firmware",
			"iwl4965-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6000-firmware",
			"iwl6000g2a-firmware",
			"iwl6050-firmware",
			"kernel-firmware",
			"libertas-usb8388-firmware",
			"ql2100-firmware",
			"ql2200-firmware",
			"ql23xx-firmware",
			"ql2400-firmware",
			"ql2500-firmware",
			"rt61pci-firmware",
			"rt73usb-firmware",
			"xorg-x11-drv-ati-firmware",
			"zd1211-firmware",
			// RHBZ#2075815
			"qemu-guest-agent",
		},
	}.Append(distroSpecificPackageSet(t))
}

// GCE BYOS image
func gcePackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	return gceCommonPackageSet(t)
}

// GCE RHUI image
func gceRhuiPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"google-rhui-client-rhel8",
		},
	}.Append(gceCommonPackageSet(t))
}
