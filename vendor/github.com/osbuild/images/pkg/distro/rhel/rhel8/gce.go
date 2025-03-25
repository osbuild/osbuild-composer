package rhel8

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
)

func gceKernelOptions() []string {
	return []string{"net.ifnames=0", "biosdevname=0", "scsi_mod.use_blk_mq=Y", "crashkernel=auto", "console=ttyS0,38400n8d"}
}

func mkGceImgType(rd distro.Distro) *rhel.ImageType {
	it := rhel.NewImageType(
		"gce",
		"image.tar.gz",
		"application/gzip",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "archive"},
		[]string{"archive"},
	)

	it.DefaultImageConfig = defaultGceByosImageConfig(rd)
	it.KernelOptions = gceKernelOptions()
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
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "archive"},
		[]string{"archive"},
	)

	it.DefaultImageConfig = defaultGceRhuiImageConfig(rd)
	it.KernelOptions = gceKernelOptions()
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
		DefaultKernel:       common.ToPtr("kernel-core"),
		UpdateDefaultKernel: common.ToPtr(true),
		// XXX: ensure the "old" behavior is preserved (that is
		// likely a bug) where for GCE the sysconfig network
		// options are not set because the merge of imageConfig
		// is shallow and the previous setup was changing the
		// kernel without also changing the network options.
		Sysconfig: &distro.Sysconfig{},
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
