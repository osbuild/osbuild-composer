package rhel8

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// PACKAGE SETS

func vhdCommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			// Defaults
			"@Core",
			"langpacks-en",

			// From the lorax kickstart
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"gdisk",
			"net-tools",
			"python3",
			"selinux-policy-targeted",
			"WALinuxAgent",

			// removed from defaults but required to boot in azure
			"dhcp-client",
		},
		Exclude: []string{
			"dracut-config-rescue",
			"rng-tools",
		},
	}.Append(bootPackageSet(t))
}

func azureRhuiCommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@Server",
			"NetworkManager",
			"NetworkManager-cloud-setup",
			"kernel",
			"kernel-core",
			"kernel-modules",
			"selinux-policy-targeted",
			"efibootmgr",
			"lvm2",
			"grub2-efi-x64",
			"shim-x64",
			"dracut-config-generic",
			"dracut-norescue",
			"bzip2",
			"langpacks-en",
			"grub2-pc",
			"rhc",
			"yum-utils",
			"rhui-azure-rhel8",
			"WALinuxAgent",
			"cloud-init",
			"cloud-utils-growpart",
			"gdisk",
			"hyperv-daemons",
			"nvme-cli",
			"cryptsetup-reencrypt",
			"uuid",
			"rng-tools",
			"patch",
		},
		Exclude: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-lib",
			"alsa-sof-firmware",
			"alsa-tools-firmware",
			"dracut-config-rescue",
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
			"libertas-sd8686-firmware",
			"libertas-sd8787-firmware",
			"libertas-usb8388-firmware",
			"glibc-all-langpacks",
			"biosdevname",
			"cockpit-podman",
			"bolt",
			"buildah",
			"containernetworking-plugins",
			"dnf-plugin-spacewalk",
			"iprutils",
			"plymouth",
			"podman",
			"python3-dnf-plugin-spacewalk",
			"python3-rhnlib",
			"python3-hwdata",
			"NetworkManager-config-server",
			"rhn-client-tools",
			"rhn-setup",
			"rhnsd",
			"rhn-check",
			"rhnlib",
			"usb_modeswitch",
		},
	}.Append(bootPackageSet(t))
}

// PARTITION TABLES

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
							Size: 10 * 1024 * 1024 * 1024,
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

// PIPELINE GENERATORS

func vhdPipelines(compress bool) pipelinesFunc {
	return func(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, containers []container.Spec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
		pipelines := make([]osbuild.Pipeline, 0)
		pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey], t.arch.distro.runner))

		partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
		if err != nil {
			return nil, err
		}

		treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], containers, customizations, options, partitionTable)
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

		qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, qemufile, osbuild.QEMUFormatVPC, nil)
		pipelines = append(pipelines, *qemuPipeline)

		if compress {
			lastPipeline := pipelines[len(pipelines)-1]
			pipelines = append(pipelines, *xzArchivePipeline(lastPipeline.Name, qemufile, t.Filename()))
		}

		return pipelines, nil
	}
}

// IMAGE DEFINITIONS

var vhdImgType = imageType{
	name:     "vhd",
	filename: "disk.vhd",
	mimeType: "application/x-vhd",
	packageSets: map[string]packageSetFunc{
		buildPkgsKey: distroBuildPackageSet,
		osPkgsKey:    vhdCommonPackageSet,
	},
	packageSetChains: map[string][]string{
		osPkgsKey: {osPkgsKey, blueprintPkgsKey},
	},
	defaultImageConfig: &distro.ImageConfig{
		EnabledServices: []string{
			"sshd",
			"waagent",
		},
		DefaultTarget: common.StringToPtr("multi-user.target"),
	},
	kernelOptions:       "ro biosdevname=0 rootdelay=300 console=ttyS0 earlyprintk=ttyS0 net.ifnames=0",
	bootable:            true,
	defaultSize:         4 * common.GibiByte,
	pipelines:           vhdPipelines(false),
	buildPipelines:      []string{"build"},
	payloadPipelines:    []string{"os", "image", "vpc"},
	exports:             []string{"vpc"},
	basePartitionTables: defaultBasePartitionTables,
}

var azureRhuiImgType = imageType{
	name:     "azure-rhui",
	filename: "disk.vhd.xz",
	mimeType: "application/xz",
	packageSets: map[string]packageSetFunc{
		buildPkgsKey: ec2BuildPackageSet,
		osPkgsKey:    azureRhuiCommonPackageSet,
	},
	packageSetChains: map[string][]string{
		osPkgsKey: {osPkgsKey, blueprintPkgsKey},
	},
	defaultImageConfig: &distro.ImageConfig{
		Timezone: common.StringToPtr("Etc/UTC"),
		Locale:   common.StringToPtr("en_US.UTF-8"),
		GPGKeyFiles: []string{
			"/etc/pki/rpm-gpg/RPM-GPG-KEY-microsoft-azure-release",
			"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
		},
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
			X11Keymap: &osbuild.X11KeymapOptions{
				Layouts: []string{"us"},
			},
		},
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
			"firewalld",
			"nm-cloud-setup.service",
			"nm-cloud-setup.timer",
			"sshd",
			"systemd-resolved",
			"waagent",
		},
		SshdConfig: &osbuild.SshdConfigStageOptions{
			Config: osbuild.SshdConfigConfig{
				ClientAliveInterval: common.IntToPtr(180),
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
				DnfPlugins: &osbuild.RHSMStageOptionsDnfPlugins{
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
		SystemdUnit: []*osbuild.SystemdUnitStageOptions{
			{
				Unit:   "nm-cloud-setup.service",
				Dropin: "10-rh-enable-for-azure.conf",
				Config: osbuild.SystemdServiceUnitDropin{
					Service: &osbuild.SystemdUnitServiceSection{
						Environment: "NM_CLOUD_SETUP_AZURE=yes",
					},
				},
			},
		},
		DefaultTarget: common.StringToPtr("multi-user.target"),
	},
	kernelOptions:       "ro crashkernel=auto console=tty1 console=ttyS0 earlyprintk=ttyS0 rootdelay=300",
	bootable:            true,
	defaultSize:         68719476736,
	pipelines:           vhdPipelines(true),
	buildPipelines:      []string{"build"},
	payloadPipelines:    []string{"os", "image", "vpc", "archive"},
	exports:             []string{"archive"},
	basePartitionTables: azureRhuiBasePartitionTables,
}
