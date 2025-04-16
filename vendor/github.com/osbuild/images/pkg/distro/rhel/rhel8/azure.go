package rhel8

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/shell"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
)

// use loglevel=3 as described in the RHEL documentation and used in existing RHEL images built by MSFT
func defaultAzureKernelOptions() []string {
	return []string{"ro", "loglevel=3", "crashkernel=auto", "console=tty1", "console=ttyS0", "earlyprintk=ttyS0", "rootdelay=300"}
}

func mkAzureRhuiImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"azure-rhui",
		"disk.vhd.xz",
		"application/xz",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vpc", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.DefaultImageConfig = defaultAzureRhuiImageConfig.InheritFrom(defaultVhdImageConfig())
	it.KernelOptions = defaultAzureKernelOptions()
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.BasePartitionTables = azureRhuiBasePartitionTables

	return it
}

func mkAzureSapRhuiImgType(rd *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"azure-sap-rhui",
		"disk.vhd.xz",
		"application/xz",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vpc", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.DefaultImageConfig = defaultAzureRhuiImageConfig.InheritFrom(sapAzureImageConfig(rd))
	it.KernelOptions = defaultAzureKernelOptions()
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.BasePartitionTables = azureRhuiBasePartitionTables

	return it
}

func mkAzureByosImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"vhd",
		"disk.vhd",
		"application/x-vhd",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vpc"},
		[]string{"vpc"},
	)

	it.DefaultImageConfig = defaultAzureByosImageConfig.InheritFrom(defaultVhdImageConfig())
	it.KernelOptions = defaultAzureKernelOptions()
	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// Azure non-RHEL image type
func mkAzureImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"vhd",
		"disk.vhd",
		"application/x-vhd",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vpc"},
		[]string{"vpc"},
	)

	it.DefaultImageConfig = defaultVhdImageConfig()
	it.KernelOptions = defaultAzureKernelOptions()
	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkAzureEap7RhuiImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"azure-eap7-rhui",
		"disk.vhd.xz",
		"application/xz",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vpc", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.DefaultImageConfig = defaultAzureEapImageConfig.InheritFrom(defaultAzureRhuiImageConfig.InheritFrom(defaultAzureImageConfig))
	it.KernelOptions = defaultAzureKernelOptions()
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.BasePartitionTables = azureRhuiBasePartitionTables
	it.Workload = eapWorkload()

	return it
}

// PARTITION TABLES

func azureRhuiBasePartitionTables(t *rhel.ImageType) (disk.PartitionTable, bool) {
	switch t.Arch().Name() {
	case arch.ARCH_X86_64.String():
		return disk.PartitionTable{
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: disk.PT_GPT,
			Size: 64 * datasizes.GibiByte,
			Partitions: []disk.Partition{
				{
					Size: 500 * datasizes.MebiByte,
					Type: disk.EFISystemPartitionGUID,
					UUID: disk.EFISystemPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "vfat",
						UUID:         disk.EFIFilesystemUUID,
						Mountpoint:   "/boot/efi",
						FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
						FSTabFreq:    0,
						FSTabPassNo:  2,
					},
				},
				{
					Size: 500 * datasizes.MebiByte,
					Type: disk.FilesystemDataGUID,
					UUID: disk.DataPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Mountpoint:   "/boot",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
				{
					Size:     2 * datasizes.MebiByte,
					Bootable: true,
					Type:     disk.BIOSBootPartitionGUID,
					UUID:     disk.BIOSBootPartitionUUID,
				},
				{
					Type: disk.LVMPartitionGUID,
					UUID: disk.RootPartitionUUID,
					Payload: &disk.LVMVolumeGroup{
						Name:        "rootvg",
						Description: "built with lvm2 and osbuild",
						LogicalVolumes: []disk.LVMLogicalVolume{
							{
								Size: 1 * datasizes.GibiByte,
								Name: "homelv",
								Payload: &disk.Filesystem{
									Type:         "xfs",
									Label:        "home",
									Mountpoint:   "/home",
									FSTabOptions: "defaults",
									FSTabFreq:    0,
									FSTabPassNo:  0,
								},
							},
							{
								Size: 2 * datasizes.GibiByte,
								Name: "rootlv",
								Payload: &disk.Filesystem{
									Type:         "xfs",
									Label:        "root",
									Mountpoint:   "/",
									FSTabOptions: "defaults",
									FSTabFreq:    0,
									FSTabPassNo:  0,
								},
							},
							{
								Size: 2 * datasizes.GibiByte,
								Name: "tmplv",
								Payload: &disk.Filesystem{
									Type:         "xfs",
									Label:        "tmp",
									Mountpoint:   "/tmp",
									FSTabOptions: "defaults",
									FSTabFreq:    0,
									FSTabPassNo:  0,
								},
							},
							{
								Size: 10 * datasizes.GibiByte,
								Name: "usrlv",
								Payload: &disk.Filesystem{
									Type:         "xfs",
									Label:        "usr",
									Mountpoint:   "/usr",
									FSTabOptions: "defaults",
									FSTabFreq:    0,
									FSTabPassNo:  0,
								},
							},
							{
								Size: 10 * datasizes.GibiByte,
								Name: "varlv",
								Payload: &disk.Filesystem{
									Type:         "xfs",
									Label:        "var",
									Mountpoint:   "/var",
									FSTabOptions: "defaults",
									FSTabFreq:    0,
									FSTabPassNo:  0,
								},
							},
						},
					},
				},
			},
		}, true

	case arch.ARCH_AARCH64.String():
		return disk.PartitionTable{
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: disk.PT_GPT,
			Size: 64 * datasizes.GibiByte,
			Partitions: []disk.Partition{
				{
					Size: 500 * datasizes.MebiByte,
					Type: disk.EFISystemPartitionGUID,
					UUID: disk.EFISystemPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "vfat",
						UUID:         disk.EFIFilesystemUUID,
						Mountpoint:   "/boot/efi",
						FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
						FSTabFreq:    0,
						FSTabPassNo:  2,
					},
				},
				{
					Size: 500 * datasizes.MebiByte,
					Type: disk.FilesystemDataGUID,
					UUID: disk.DataPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Mountpoint:   "/boot",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
				{
					Type: disk.LVMPartitionGUID,
					UUID: disk.RootPartitionUUID,
					Payload: &disk.LVMVolumeGroup{
						Name:        "rootvg",
						Description: "built with lvm2 and osbuild",
						LogicalVolumes: []disk.LVMLogicalVolume{
							{
								Size: 1 * datasizes.GibiByte,
								Name: "homelv",
								Payload: &disk.Filesystem{
									Type:         "xfs",
									Label:        "home",
									Mountpoint:   "/home",
									FSTabOptions: "defaults",
									FSTabFreq:    0,
									FSTabPassNo:  0,
								},
							},
							{
								Size: 2 * datasizes.GibiByte,
								Name: "rootlv",
								Payload: &disk.Filesystem{
									Type:         "xfs",
									Label:        "root",
									Mountpoint:   "/",
									FSTabOptions: "defaults",
									FSTabFreq:    0,
									FSTabPassNo:  0,
								},
							},
							{
								Size: 2 * datasizes.GibiByte,
								Name: "tmplv",
								Payload: &disk.Filesystem{
									Type:         "xfs",
									Label:        "tmp",
									Mountpoint:   "/tmp",
									FSTabOptions: "defaults",
									FSTabFreq:    0,
									FSTabPassNo:  0,
								},
							},
							{
								Size: 10 * datasizes.GibiByte,
								Name: "usrlv",
								Payload: &disk.Filesystem{
									Type:         "xfs",
									Label:        "usr",
									Mountpoint:   "/usr",
									FSTabOptions: "defaults",
									FSTabFreq:    0,
									FSTabPassNo:  0,
								},
							},
							{
								Size: 10 * datasizes.GibiByte,
								Name: "varlv",
								Payload: &disk.Filesystem{
									Type:         "xfs",
									Label:        "var",
									Mountpoint:   "/var",
									FSTabOptions: "defaults",
									FSTabFreq:    0,
									FSTabPassNo:  0,
								},
							},
						},
					},
				},
			},
		}, true

	default:
		return disk.PartitionTable{}, false
	}
}

// based on https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/8/html/deploying_rhel_8_on_microsoft_azure/assembly_deploying-a-rhel-image-as-a-virtual-machine-on-microsoft-azure_cloud-content-azure#making-configuration-changes_configure-the-image-azure
var defaultAzureImageConfig = &distro.ImageConfig{
	Timezone: common.ToPtr("Etc/UTC"),
	Locale:   common.ToPtr("en_US.UTF-8"),
	Keyboard: &osbuild.KeymapStageOptions{
		Keymap: "us",
		X11Keymap: &osbuild.X11KeymapOptions{
			Layouts: []string{"us"},
		},
	},
	DefaultKernel:       common.ToPtr("kernel-core"),
	UpdateDefaultKernel: common.ToPtr(true),
	Sysconfig: &distro.Sysconfig{
		Networking: true,
		NoZeroConf: true,
	},
	EnabledServices: []string{
		"nm-cloud-setup.service",
		"nm-cloud-setup.timer",
		"sshd",
		"waagent",
	},
	SshdConfig: &osbuild.SshdConfigStageOptions{
		Config: osbuild.SshdConfigConfig{
			ClientAliveInterval: common.ToPtr(180),
		},
	},
	Modprobe: []*osbuild.ModprobeStageOptions{
		{
			Filename: "blacklist-amdgpu.conf",
			Commands: osbuild.ModprobeConfigCmdList{
				osbuild.NewModprobeConfigCmdBlacklist("amdgpu"),
			},
		},
		{
			Filename: "blacklist-intel-cstate.conf",
			Commands: osbuild.ModprobeConfigCmdList{
				osbuild.NewModprobeConfigCmdBlacklist("intel_cstate"),
			},
		},
		{
			Filename: "blacklist-floppy.conf",
			Commands: osbuild.ModprobeConfigCmdList{
				osbuild.NewModprobeConfigCmdBlacklist("floppy"),
			},
		},
		{
			Filename: "blacklist-nouveau.conf",
			Commands: osbuild.ModprobeConfigCmdList{
				osbuild.NewModprobeConfigCmdBlacklist("nouveau"),
				osbuild.NewModprobeConfigCmdBlacklist("lbm-nouveau"),
			},
		},
		{
			Filename: "blacklist-skylake-edac.conf",
			Commands: osbuild.ModprobeConfigCmdList{
				osbuild.NewModprobeConfigCmdBlacklist("skx_edac"),
			},
		},
	},
	CloudInit: []*osbuild.CloudInitStageOptions{
		{
			Filename: "10-azure-kvp.cfg",
			Config: osbuild.CloudInitConfigFile{
				Reporting: &osbuild.CloudInitConfigReporting{
					Logging: &osbuild.CloudInitConfigReportingHandlers{
						Type: "log",
					},
					Telemetry: &osbuild.CloudInitConfigReportingHandlers{
						Type: "hyperv",
					},
				},
			},
		},
		{
			Filename: "91-azure_datasource.cfg",
			Config: osbuild.CloudInitConfigFile{
				Datasource: &osbuild.CloudInitConfigDatasource{
					Azure: &osbuild.CloudInitConfigDatasourceAzure{
						ApplyNetworkConfig: false,
					},
				},
				DatasourceList: []string{
					"Azure",
				},
			},
		},
	},
	PwQuality: &osbuild.PwqualityConfStageOptions{
		Config: osbuild.PwqualityConfConfig{
			Minlen:   common.ToPtr(6),
			Minclass: common.ToPtr(3),
			Dcredit:  common.ToPtr(0),
			Ucredit:  common.ToPtr(0),
			Lcredit:  common.ToPtr(0),
			Ocredit:  common.ToPtr(0),
		},
	},
	WAAgentConfig: &osbuild.WAAgentConfStageOptions{
		Config: osbuild.WAAgentConfig{
			RDFormat:     common.ToPtr(false),
			RDEnableSwap: common.ToPtr(false),
		},
	},
	Grub2Config: &osbuild.GRUB2Config{
		DisableRecovery: common.ToPtr(true),
		DisableSubmenu:  common.ToPtr(true),
		Distributor:     "$(sed 's, release .*$,,g' /etc/system-release)",
		Terminal:        []string{"serial", "console"},
		Serial:          "serial --speed=115200 --unit=0 --word=8 --parity=no --stop=1",
		Timeout:         10,
		TimeoutStyle:    osbuild.GRUB2ConfigTimeoutStyleCountdown,
	},
	UdevRules: &osbuild.UdevRulesStageOptions{
		Filename: "/etc/udev/rules.d/68-azure-sriov-nm-unmanaged.rules",
		Rules: osbuild.UdevRules{
			osbuild.UdevRuleComment{
				Comment: []string{
					"Accelerated Networking on Azure exposes a new SRIOV interface to the VM.",
					"This interface is transparently bonded to the synthetic interface,",
					"so NetworkManager should just ignore any SRIOV interfaces.",
				},
			},
			osbuild.NewUdevRule(
				[]osbuild.UdevKV{
					{K: "SUBSYSTEM", O: "==", V: "net"},
					{K: "DRIVERS", O: "==", V: "hv_pci"},
					{K: "ACTION", O: "==", V: "add"},
					{K: "ENV", A: "NM_UNMANAGED", O: "=", V: "1"},
				},
			),
		},
	},
	SystemdDropin: []*osbuild.SystemdUnitStageOptions{
		{
			Unit:   "nm-cloud-setup.service",
			Dropin: "10-rh-enable-for-azure.conf",
			Config: osbuild.SystemdServiceUnitDropin{
				Service: &osbuild.SystemdUnitServiceSection{
					Environment: []osbuild.EnvironmentVariable{{Key: "NM_CLOUD_SETUP_AZURE", Value: "yes"}},
				},
			},
		},
	},
	DefaultTarget: common.ToPtr("multi-user.target"),
}

// Diff of the default Image Config compare to the `defaultAzureImageConfig`
// The configuration for non-RHUI images does not touch the RHSM configuration at all.
// https://issues.redhat.com/browse/COMPOSER-2157
var defaultAzureByosImageConfig = &distro.ImageConfig{
	GPGKeyFiles: []string{
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
	},
}

// Diff of the default Image Config compare to the `defaultAzureImageConfig`
var defaultAzureRhuiImageConfig = &distro.ImageConfig{
	GPGKeyFiles: []string{
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-microsoft-azure-release",
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
	},
	RHSMConfig: map[subscription.RHSMStatus]*subscription.RHSMConfig{
		subscription.RHSMConfigNoSubscription: {
			DnfPlugins: subscription.SubManDNFPluginsConfig{
				SubscriptionManager: subscription.DNFPluginConfig{
					Enabled: common.ToPtr(false),
				},
			},
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

const wildflyPath = "/opt/rh/eap7/root/usr/share/wildfly"

var defaultAzureEapImageConfig = &distro.ImageConfig{
	// shell env vars for EAP
	ShellInit: []shell.InitFile{
		{
			Filename: "eap_env.sh",
			Variables: []shell.EnvironmentVariable{
				{
					Key:   "EAP_HOME",
					Value: wildflyPath,
				},
				{
					Key:   "JBOSS_HOME",
					Value: wildflyPath,
				},
			},
		},
	},
}

func defaultVhdImageConfig() *distro.ImageConfig {
	imageConfig := &distro.ImageConfig{
		EnabledServices: append(defaultAzureImageConfig.EnabledServices, "firewalld"),
	}
	return imageConfig.InheritFrom(defaultAzureImageConfig)
}

func sapAzureImageConfig(rd *rhel.Distribution) *distro.ImageConfig {
	return sapImageConfig(rd).InheritFrom(defaultVhdImageConfig())
}
