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

// Common Azure image package set
func azureCommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
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
			"yum-utils",
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
	}.Append(bootPackageSet(t)).Append(distroSpecificPackageSet(t))

	if t.arch.distro.isRHEL() {
		ps.Append(rpmmd.PackageSet{
			Include: []string{
				"insights-client",
				"rhc",
			},
		})
	}

	return ps
}

// Azure BYOS image package set
func azurePackageSet(t *imageType) rpmmd.PackageSet {
	return azureCommonPackageSet(t)
}

// Azure RHUI image package set
func azureRhuiPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"rhui-azure-rhel8",
		},
	}.Append(azureCommonPackageSet(t))
}

// PARTITION TABLES

var azureRhuiBasePartitionTables = distro.BasePartitionTableMap{
	distro.X86_64ArchName: disk.PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Size: 64 * common.GibiByte,
		Partitions: []disk.Partition{
			{
				Size: 500 * common.MebiByte,
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
				Size: 500 * common.MebiByte,
				Type: disk.FilesystemDataGUID,
				UUID: disk.FilesystemDataUUID,
				Payload: &disk.Filesystem{
					Type:         defaultFSType,
					Mountpoint:   "/boot",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
			{
				Size:     2 * common.MebiByte,
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
							Size: 1 * common.GibiByte,
							Name: "homelv",
							Payload: &disk.Filesystem{
								Type:         defaultFSType,
								Label:        "home",
								Mountpoint:   "/home",
								FSTabOptions: "defaults",
								FSTabFreq:    0,
								FSTabPassNo:  0,
							},
						},
						{
							Size: 2 * common.GibiByte,
							Name: "rootlv",
							Payload: &disk.Filesystem{
								Type:         defaultFSType,
								Label:        "root",
								Mountpoint:   "/",
								FSTabOptions: "defaults",
								FSTabFreq:    0,
								FSTabPassNo:  0,
							},
						},
						{
							Size: 2 * common.GibiByte,
							Name: "tmplv",
							Payload: &disk.Filesystem{
								Type:         defaultFSType,
								Label:        "tmp",
								Mountpoint:   "/tmp",
								FSTabOptions: "defaults",
								FSTabFreq:    0,
								FSTabPassNo:  0,
							},
						},
						{
							Size: 10 * common.GibiByte,
							Name: "usrlv",
							Payload: &disk.Filesystem{
								Type:         defaultFSType,
								Label:        "usr",
								Mountpoint:   "/usr",
								FSTabOptions: "defaults",
								FSTabFreq:    0,
								FSTabPassNo:  0,
							},
						},
						{
							Size: 10 * common.GibiByte,
							Name: "varlv",
							Payload: &disk.Filesystem{
								Type:         defaultFSType,
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

		partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(defaultFSType), options, rng)
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

var defaultAzureKernelOptions = "ro crashkernel=auto console=tty1 console=ttyS0 earlyprintk=ttyS0 rootdelay=300"

var defaultAzureImageConfig = &distro.ImageConfig{
	Timezone: common.StringToPtr("Etc/UTC"),
	Locale:   common.StringToPtr("en_US.UTF-8"),
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
}

// Azure non-RHEL image type
var azureImgType = imageType{
	name:     "vhd",
	filename: "disk.vhd",
	mimeType: "application/x-vhd",
	packageSets: map[string]packageSetFunc{
		// the ec2 buildroot is required due to the cloud-init stage and dependency on YAML
		buildPkgsKey: ec2BuildPackageSet,
		osPkgsKey:    azurePackageSet,
	},
	packageSetChains: map[string][]string{
		osPkgsKey: {osPkgsKey, blueprintPkgsKey},
	},
	defaultImageConfig:  defaultAzureImageConfig,
	kernelOptions:       defaultAzureKernelOptions,
	bootable:            true,
	defaultSize:         4 * common.GibiByte,
	pipelines:           vhdPipelines(false),
	buildPipelines:      []string{"build"},
	payloadPipelines:    []string{"os", "image", "vpc"},
	exports:             []string{"vpc"},
	basePartitionTables: defaultBasePartitionTables,
}

// Diff of the default Image Config compare to the `defaultAzureImageConfig`
var defaultAzureByosImageConfig = &distro.ImageConfig{
	GPGKeyFiles: []string{
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
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
}

// Azure BYOS image type
var azureByosImgType = imageType{
	name:     "vhd",
	filename: "disk.vhd",
	mimeType: "application/x-vhd",
	packageSets: map[string]packageSetFunc{
		// the ec2 buildroot is required due to the cloud-init stage and dependency on YAML
		buildPkgsKey: ec2BuildPackageSet,
		osPkgsKey:    azurePackageSet,
	},
	packageSetChains: map[string][]string{
		osPkgsKey: {osPkgsKey, blueprintPkgsKey},
	},
	defaultImageConfig:  defaultAzureByosImageConfig.InheritFrom(defaultAzureImageConfig),
	kernelOptions:       defaultAzureKernelOptions,
	bootable:            true,
	defaultSize:         4 * common.GibiByte,
	pipelines:           vhdPipelines(false),
	buildPipelines:      []string{"build"},
	payloadPipelines:    []string{"os", "image", "vpc"},
	exports:             []string{"vpc"},
	basePartitionTables: defaultBasePartitionTables,
}

// Diff of the default Image Config compare to the `defaultAzureImageConfig`
var defaultAzureRhuiImageConfig = &distro.ImageConfig{
	GPGKeyFiles: []string{
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-microsoft-azure-release",
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
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
}

var azureRhuiImgType = imageType{
	name:     "azure-rhui",
	filename: "disk.vhd.xz",
	mimeType: "application/xz",
	packageSets: map[string]packageSetFunc{
		// the ec2 buildroot is required due to the cloud-init stage and dependency on YAML
		buildPkgsKey: ec2BuildPackageSet,
		osPkgsKey:    azureRhuiPackageSet,
	},
	packageSetChains: map[string][]string{
		osPkgsKey: {osPkgsKey, blueprintPkgsKey},
	},
	defaultImageConfig:  defaultAzureRhuiImageConfig.InheritFrom(defaultAzureImageConfig),
	kernelOptions:       defaultAzureKernelOptions,
	bootable:            true,
	defaultSize:         68719476736,
	pipelines:           vhdPipelines(true),
	buildPipelines:      []string{"build"},
	payloadPipelines:    []string{"os", "image", "vpc", "archive"},
	exports:             []string{"archive"},
	basePartitionTables: azureRhuiBasePartitionTables,
}
