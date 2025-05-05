package rhel10

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
)

func gceKernelOptions() []string {
	return []string{"biosdevname=0", "scsi_mod.use_blk_mq=Y", "console=ttyS0,38400n8d"}
}

func mkGCEImageType() *rhel.ImageType {
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

	it.DefaultImageConfig = baseGCEImageConfig()
	it.DefaultImageConfig.KernelOptions = gceKernelOptions()
	it.DefaultSize = 20 * datasizes.GibiByte
	it.Bootable = true
	// TODO: the base partition table still contains the BIOS boot partition, but the image is UEFI-only
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func baseGCEImageConfig() *distro.ImageConfig {
	ic := &distro.ImageConfig{
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
			// TODO: remove cloud-init services once we switch back to GCP guest tools
			"cloud-init",
			"cloud-init-local",
			"cloud-config",
			"cloud-final",
		},
		DisabledServices: []string{
			"sshd-keygen@",
			"reboot.target",
		},
		DefaultTarget: common.ToPtr("multi-user.target"),
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
						Id:   "google-compute-engine",
						Name: "Google Compute Engine",
						// TODO: use el10 repo once it's available
						BaseURLs: []string{"https://packages.cloud.google.com/yum/repos/google-compute-engine-el9-x86_64-stable"},
						Enabled:  common.ToPtr(true),
						// TODO: enable GPG check once Google stops using SHA-1 in their keys
						// https://issuetracker.google.com/issues/360905189
						GPGCheck:     common.ToPtr(false),
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
		UpdateDefaultKernel: common.ToPtr(true),
		DefaultKernel:       common.ToPtr("kernel-core"),
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

	return ic
}
