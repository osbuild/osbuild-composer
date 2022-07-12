package rhel7

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func azureRhuiCommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@base",
			"@core",
			"authconfig",
			"bpftool",
			"bzip2",
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"dracut-config-generic",
			"dracut-norescue",
			"efibootmgr",
			"firewalld",
			"gdisk",
			"grub2-efi-x64",
			"grub2-pc",
			"grub2",
			"hyperv-daemons",
			"kernel",
			"lvm2",
			"redhat-release-eula",
			"redhat-support-tool",
			"rh-dotnetcore11",
			"rhn-setup",
			"rhui-azure-rhel7",
			"rsync",
			"shim-x64",
			"tar",
			"tcpdump",
			"WALinuxAgent",
			"yum-rhn-plugin",
			"yum-utils",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"mariadb-libs",
			"NetworkManager-config-server",
			"postfix",
		},
	}.Append(bootPackageSet(t))
}

var azureRhuiBasePartitionTables = distro.BasePartitionTableMap{
	distro.X86_64ArchName: disk.PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Size: 68719476736,
		Partitions: []disk.Partition{
			{
				Size: 524288000,
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
				Size: 524288000,
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Payload: &disk.Filesystem{
					Type:         "xfs",
					Mountpoint:   "/boot",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
			{
				Size:     2097152,
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
							Size: 1 * 1024 * 1024 * 1024,
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
							Size: 2 * 1024 * 1024 * 1024,
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
							Size: 2 * 1024 * 1024 * 1024,
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
							Size: 10 * 1024 * 1024 * 1024,
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
							Size: 10 * 1024 * 1024 * 1024, // firedrill: 8 GB
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
	},
}

func vhdPipelines(compress bool) pipelinesFunc {
	return func(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
		pipelines := make([]osbuild.Pipeline, 0)
		pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey], t.arch.distro.runner))

		partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
		if err != nil {
			return nil, err
		}

		treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, partitionTable)
		if err != nil {
			return nil, err
		}
		pipelines = append(pipelines, *treePipeline)

		diskfile := "disk.img"
		kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
		imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
		pipelines = append(pipelines, *imagePipeline)
		if err != nil {
			return nil, err
		}

		var qemufile string
		if compress {
			qemufile = "disk.vhd"
		} else {
			qemufile = t.filename
		}

		qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, qemufile, osbuild.QEMUFormatVPC, osbuild.VPCOptions{ForceSize: common.BoolToPtr(false)})
		pipelines = append(pipelines, *qemuPipeline)

		if compress {
			lastPipeline := pipelines[len(pipelines)-1]
			pipelines = append(pipelines, *xzArchivePipeline(lastPipeline.Name, qemufile, t.Filename()))
		}

		return pipelines, nil
	}
}

var azureRhuiImgType = imageType{
	name:     "azure-rhui",
	filename: "disk.vhd.xz",
	mimeType: "application/xz",
	packageSets: map[string]packageSetFunc{
		buildPkgsKey: distroBuildPackageSet,
		osPkgsKey:    azureRhuiCommonPackageSet,
	},
	packageSetChains: map[string][]string{
		osPkgsKey: {osPkgsKey, blueprintPkgsKey},
	},
	defaultImageConfig: &distro.ImageConfig{
		Timezone: "Etc/UTC",
		Locale:   "en_US.UTF-8",
		GPGKeyFiles: []string{
			"/etc/pki/rpm-gpg/RPM-GPG-KEY-microsoft-azure-release",
			"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
		},
		Authconfig: &osbuild.AuthconfigStageOptions{},
		Sysconfig: []*osbuild.SysconfigStageOptions{
			{
				Kernel: &osbuild.SysconfigKernelOptions{
					UpdateDefault: true,
					DefaultKernel: "kernel-core",
				},
				Network: &osbuild.SysconfigNetworkOptions{
					Networking: true,
					NoZeroConf: true,
				},
			},
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
				ClientAliveInterval: common.IntToPtr(180),
			},
		},
		Modprobe: []*osbuild.ModprobeStageOptions{
			{
				Filename: "blacklist-nouveau.conf",
				Commands: osbuild.ModprobeConfigCmdList{
					osbuild.NewModprobeConfigCmdBlacklist("nouveau"),
					osbuild.NewModprobeConfigCmdBlacklist("lbm-nouveau"),
				},
			},
			{
				Filename: "blacklist-floppy.conf",
				Commands: osbuild.ModprobeConfigCmdList{
					osbuild.NewModprobeConfigCmdBlacklist("floppy"),
				},
			},
		},
		CloudInit: []*osbuild.CloudInitStageOptions{
			{
				Filename: "06_logging_override.cfg",
				Config: osbuild.CloudInitConfigFile{
					Output: &osbuild.CloudInitConfigOutput{
						All: common.StringToPtr("| tee -a /var/log/cloud-init-output.log"),
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
				Minlen:   common.IntToPtr(6),
				Minclass: common.IntToPtr(3),
				Dcredit:  common.IntToPtr(0),
				Ucredit:  common.IntToPtr(0),
				Lcredit:  common.IntToPtr(0),
				Ocredit:  common.IntToPtr(0),
			},
		},
		WAAgentConfig: &osbuild.WAAgentConfStageOptions{
			Config: osbuild.WAAgentConfig{
				RDFormat:     common.BoolToPtr(false),
				RDEnableSwap: common.BoolToPtr(false),
			},
		},
		RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
			distro.RHSMConfigNoSubscription: {
				YumPlugins: &osbuild.RHSMStageOptionsDnfPlugins{
					SubscriptionManager: &osbuild.RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
				},
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					Rhsm: &osbuild.SubManConfigRHSMSection{
						ManageRepos: common.BoolToPtr(false),
					},
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
				HttpCaching: common.StringToPtr("packages"),
			},
			Plugins: &osbuild.YumConfigPlugins{
				Langpacks: &osbuild.YumConfigPluginsLangpacks{
					Locales: []string{"en_US.UTF-8"},
				},
			},
		},
		DefaultTarget: "multi-user.target",
	},
	kernelOptions:       "ro crashkernel=auto console=tty1 console=ttyS0 earlyprintk=ttyS0 rootdelay=300 scsi_mod.use_blk_mq=y",
	bootable:            true,
	defaultSize:         68719476736,
	pipelines:           vhdPipelines(true),
	buildPipelines:      []string{"build"},
	payloadPipelines:    []string{"os", "image", "vpc", "archive"},
	exports:             []string{"archive"},
	basePartitionTables: azureRhuiBasePartitionTables,
}
