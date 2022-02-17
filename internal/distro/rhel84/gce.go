package rhel84

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// common GCE image
func getGceCommonPackageSet() rpmmd.PackageSet {
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
			// GCP SDK
			"google-cloud-sdk",

			// Not explicitly included in GCP kickstart, but present on the image
			// for time synchronization
			"chrony",
			"timedatex",
			// Detected Platform requirements by Anaconda
			"qemu-guest-agent",
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
		},
	}
}

// GCE BYOS image
func getGcePackageSet() rpmmd.PackageSet {
	return getGceCommonPackageSet()
}

// gcePipelinesRhel86 is a slightly modified RHEL-86 version of gcePipelines() function
func gcePipelinesRhel86(t *imageTypeS2, imageConfig *distro.ImageConfig, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *t.buildPipeline(repos, packageSetSpecs["build-packages"]))

	partitionTable, err := t.getPartitionTable(options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline, err := osPipelineRhel86(t, imageConfig, repos, packageSetSpecs["packages"], customizations, options, partitionTable)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.raw"
	kernelVer, err := rpmmd.GetVerStrFromPackageSpecList(packageSetSpecs["packages"], customizations.GetKernel().Name)
	if err != nil {
		panic(fmt.Sprintf("kernel package %q not found", customizations.GetKernel().Name))
	}
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)

	archivePipeline := tarArchivePipeline("archive", imagePipeline.Name, &osbuild.TarStageOptions{
		Filename: t.Filename(),
		Format:   osbuild.TarArchiveFormatOldgnu,
		RootNode: osbuild.TarRootNodeOmit,
		// import of the image to GCP fails in case the options below are enabled, which is the default
		ACLs:    common.BoolToPtr(false),
		SELinux: common.BoolToPtr(false),
		Xattrs:  common.BoolToPtr(false),
	})
	pipelines = append(pipelines, *archivePipeline)

	return pipelines, nil
}

func getDefaultGceByosImageConfig() *distro.ImageConfig {
	return &distro.ImageConfig{
		Timezone: "UTC",
		TimeSynchronization: &osbuild.ChronyStageOptions{
			Timeservers: []string{"metadata.google.internal"},
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
		DefaultTarget: "multi-user.target",
		Locale:        "en_US.UTF-8",
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
					ApplyUpdates: common.BoolToPtr(true),
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
						BaseURL:      []string{"https://packages.cloud.google.com/yum/repos/google-compute-engine-el8-x86_64-stable"},
						Enabled:      common.BoolToPtr(true),
						GPGCheck:     common.BoolToPtr(true),
						RepoGPGCheck: common.BoolToPtr(false),
						GPGKey: []string{
							"https://packages.cloud.google.com/yum/doc/yum-key.gpg",
							"https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg",
						},
					},
					{
						Id:           "google-cloud-sdk",
						Name:         "Google Cloud SDK",
						BaseURL:      []string{"https://packages.cloud.google.com/yum/repos/cloud-sdk-el8-x86_64"},
						Enabled:      common.BoolToPtr(true),
						GPGCheck:     common.BoolToPtr(true),
						RepoGPGCheck: common.BoolToPtr(false),
						GPGKey: []string{
							"https://packages.cloud.google.com/yum/doc/yum-key.gpg",
							"https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg",
						},
					},
				},
			},
		},
		RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
			distro.RHSMConfigNoSubscription: {
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					// Don't disable RHSM redhat.repo management on the GCE
					// image, which is BYOS and does not use RHUI for content.
					// Otherwise subscribing the system manually after booting
					// it would result in empty redhat.repo. Without RHUI, such
					// system would have no way to get Red Hat content, but
					// enable the repo management manually, which would be very
					// confusing.
				},
			},
			distro.RHSMConfigWithSubscription: {
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					// do not disable the redhat.repo management if the user
					// explicitly request the system to be subscribed
				},
			},
		},
		SshdConfig: &osbuild.SshdConfigStageOptions{
			Config: osbuild.SshdConfigConfig{
				PasswordAuthentication: common.BoolToPtr(false),
				ClientAliveInterval:    common.IntToPtr(420),
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
	}
}

// GCE BYOS image
func gceByosPipelines(t *imageTypeS2, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	return gcePipelinesRhel86(t, getDefaultGceByosImageConfig(), customizations, options, repos, packageSetSpecs, rng)
}
