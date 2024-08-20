package rhel10

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
)

const gceKernelOptions = "biosdevname=0 scsi_mod.use_blk_mq=Y console=ttyS0,38400n8d"

func mkGCEImageType() *rhel.ImageType {
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

	it.DefaultImageConfig = baseGCEImageConfig()
	it.KernelOptions = gceKernelOptions
	it.DefaultSize = 20 * common.GibiByte
	it.Bootable = true
	// TODO: the base partition table still contains the BIOS boot partition, but the image is UEFI-only
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func baseGCEImageConfig() *distro.ImageConfig {
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

	return ic
}

func gceCommonPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"@core",
			"langpacks-en", // not in Google's KS
			"acpid",
			"dnf-automatic",
			"net-tools",
			"python3",
			"rng-tools",
			"tar",
			"vim",

			// GCE guest tools
			// TODO: uncomment once the package is available
			// the el9 version depends on libboost_regex.so.1.75.0()(64bit), which is not available on el10
			//"google-compute-engine",
			"google-osconfig-agent",
			"gce-disk-expand",
			// cloud-init is a replacement for "google-compute-engine", remove once the package is available
			"cloud-init",

			// Not explicitly included in GCP kickstart, but present on the image
			// for time synchronization
			"chrony",
			"timedatex",
			// EFI
			"grub2-tools",
			"grub2-tools-minimal",
			// Performance tuning
			"tuned",
		},
		Exclude: []string{
			"alsa-utils",
			"b43-fwcutter",
			"dmraid",
			"dracut-config-rescue",
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
			"iwl105-firmware",
			"iwl135-firmware",
			"iwl1000-firmware",
			"iwl2000-firmware",
			"iwl2030-firmware",
			"iwl3160-firmware",
			"iwl3945-firmware",
			"iwl4965-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6000-firmware",
			"iwl6000g2a-firmware",
			"iwl6050-firmware",
			"iwl7260-firmware",
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

	return ps
}

// GCE image
func gcePackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	return gceCommonPackageSet(t)
}
