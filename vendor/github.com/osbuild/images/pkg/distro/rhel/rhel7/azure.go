package rhel7

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
)

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

	// all RHEL 7 images should use sgdisk
	it.DiskImagePartTool = common.ToPtr(osbuild.PTSgdisk)
	// RHEL 7 qemu vpc subformat does not support force_size
	it.DiskImageVPCForceSize = common.ToPtr(false)

	it.Compression = "xz"
	it.KernelOptions = []string{"ro", "crashkernel=auto", "console=tty1", "console=ttyS0", "earlyprintk=ttyS0", "rootdelay=300", "scsi_mod.use_blk_mq=y"}
	it.DefaultImageConfig = azureDefaultImgConfig
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.BasePartitionTables = azureRhuiBasePartitionTables

	return it
}

var azureDefaultImgConfig = &distro.ImageConfig{
	Timezone: common.ToPtr("Etc/UTC"),
	Locale:   common.ToPtr("en_US.UTF-8"),
	GPGKeyFiles: []string{
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-microsoft-azure-release",
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
	},
	SELinuxForceRelabel: common.ToPtr(true),
	Authconfig:          &osbuild.AuthconfigStageOptions{},
	UpdateDefaultKernel: common.ToPtr(true),
	DefaultKernel:       common.ToPtr("kernel-core"),

	Sysconfig: &distro.Sysconfig{
		Networking: true,
		NoZeroConf: true,
	},
	EnabledServices: []string{
		"cloud-config",
		"cloud-final",
		"cloud-init-local",
		"cloud-init",
		"firewalld",
		"NetworkManager",
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
			Filename: "06_logging_override.cfg",
			Config: osbuild.CloudInitConfigFile{
				Output: &osbuild.CloudInitConfigOutput{
					All: common.ToPtr("| tee -a /var/log/cloud-init-output.log"),
				},
			},
		},
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
	RHSMConfig: map[subscription.RHSMStatus]*subscription.RHSMConfig{
		subscription.RHSMConfigNoSubscription: {
			YumPlugins: subscription.SubManDNFPluginsConfig{
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
	Grub2Config: &osbuild.GRUB2Config{
		TerminalInput:  []string{"serial", "console"},
		TerminalOutput: []string{"serial", "console"},
		Serial:         "serial --speed=115200 --unit=0 --word=8 --parity=no --stop=1",
		Timeout:        10,
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
	YumConfig: &osbuild.YumConfigStageOptions{
		Config: &osbuild.YumConfigConfig{
			HttpCaching: common.ToPtr("packages"),
		},
		Plugins: &osbuild.YumConfigPlugins{
			Langpacks: &osbuild.YumConfigPluginsLangpacks{
				Locales: []string{"en_US.UTF-8"},
			},
		},
	},
	DefaultTarget: common.ToPtr("multi-user.target"),
}

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

	default:
		return disk.PartitionTable{}, false
	}
}
