package rhel8

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/shell"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/subscription"
)

const defaultAzureKernelOptions = "ro crashkernel=auto console=tty1 console=ttyS0 earlyprintk=ttyS0 rootdelay=300"

func azureRhuiImgType() imageType {
	return imageType{
		name:        "azure-rhui",
		filename:    "disk.vhd.xz",
		mimeType:    "application/xz",
		compression: "xz",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: azureRhuiPackageSet,
		},
		defaultImageConfig:  defaultAzureRhuiImageConfig.InheritFrom(defaultVhdImageConfig()),
		kernelOptions:       defaultAzureKernelOptions,
		bootable:            true,
		defaultSize:         64 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vpc", "xz"},
		exports:             []string{"xz"},
		basePartitionTables: azureRhuiBasePartitionTables,
	}
}

func azureSapRhuiImgType(rd distribution) imageType {
	return imageType{
		name:        "azure-sap-rhui",
		filename:    "disk.vhd.xz",
		mimeType:    "application/xz",
		compression: "xz",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: azureSapPackageSet,
		},
		defaultImageConfig:  defaultAzureRhuiImageConfig.InheritFrom(sapAzureImageConfig(rd)),
		kernelOptions:       defaultAzureKernelOptions,
		bootable:            true,
		defaultSize:         64 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vpc", "xz"},
		exports:             []string{"xz"},
		basePartitionTables: azureRhuiBasePartitionTables,
	}
}

func azureByosImgType() imageType {
	return imageType{
		name:     "vhd",
		filename: "disk.vhd",
		mimeType: "application/x-vhd",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: azurePackageSet,
		},
		defaultImageConfig:  defaultAzureByosImageConfig.InheritFrom(defaultVhdImageConfig()),
		kernelOptions:       defaultAzureKernelOptions,
		bootable:            true,
		defaultSize:         4 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vpc"},
		exports:             []string{"vpc"},
		basePartitionTables: defaultBasePartitionTables,
	}
}

// Azure non-RHEL image type
func azureImgType() imageType {
	return imageType{
		name:     "vhd",
		filename: "disk.vhd",
		mimeType: "application/x-vhd",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: azurePackageSet,
		},
		defaultImageConfig:  defaultVhdImageConfig(),
		kernelOptions:       defaultAzureKernelOptions,
		bootable:            true,
		defaultSize:         4 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vpc"},
		exports:             []string{"vpc"},
		basePartitionTables: defaultBasePartitionTables,
	}
}

func azureEap7RhuiImgType() imageType {
	return imageType{
		name:        "azure-eap7-rhui",
		workload:    eapWorkload(),
		filename:    "disk.vhd.xz",
		mimeType:    "application/xz",
		compression: "xz",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: azureEapPackageSet,
		},
		defaultImageConfig:  defaultAzureEapImageConfig.InheritFrom(defaultAzureRhuiImageConfig.InheritFrom(defaultAzureImageConfig)),
		kernelOptions:       defaultAzureKernelOptions,
		bootable:            true,
		defaultSize:         64 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vpc", "xz"},
		exports:             []string{"xz"},
		basePartitionTables: azureRhuiBasePartitionTables,
	}
}

// PACKAGE SETS

// Common Azure image package set
func azureCommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"@Server",
			"NetworkManager",
			"NetworkManager-cloud-setup",
			"WALinuxAgent",
			"bzip2",
			"cloud-init",
			"cloud-utils-growpart",
			"cryptsetup-reencrypt",
			"dracut-config-generic",
			"dracut-norescue",
			"efibootmgr",
			"gdisk",
			"hyperv-daemons",
			"kernel",
			"kernel-core",
			"kernel-modules",
			"langpacks-en",
			"lvm2",
			"nvme-cli",
			"patch",
			"rng-tools",
			"selinux-policy-targeted",
			"uuid",
			"yum-utils",
		},
		Exclude: []string{
			"NetworkManager-config-server",
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-sof-firmware",
			"alsa-tools-firmware",
			"biosdevname",
			"bolt",
			"buildah",
			"cockpit-podman",
			"containernetworking-plugins",
			"dnf-plugin-spacewalk",
			"dracut-config-rescue",
			"glibc-all-langpacks",
			"iprutils",
			"ivtv-firmware",
			"iwl100-firmware",
			"iwl1000-firmware",
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
			"plymouth",
			"podman",
			"python3-dnf-plugin-spacewalk",
			"python3-hwdata",
			"python3-rhnlib",
			"rhn-check",
			"rhn-client-tools",
			"rhn-setup",
			"rhnlib",
			"rhnsd",
			"usb_modeswitch",
		},
	}.Append(distroSpecificPackageSet(t))

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
	return rpmmd.PackageSet{
		Include: []string{
			"firewalld",
		},
		Exclude: []string{
			"alsa-lib",
		},
	}.Append(azureCommonPackageSet(t))
}

// Azure RHUI image package set
func azureRhuiPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"firewalld",
			"rhui-azure-rhel8",
		},
		Exclude: []string{
			"alsa-lib",
		},
	}.Append(azureCommonPackageSet(t))
}

// Azure SAP image package set
// Includes the common azure package set, the common SAP packages, and
// the azure rhui sap package.
func azureSapPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"firewalld",
			"rhui-azure-rhel8-sap-ha",
		},
	}.Append(azureCommonPackageSet(t)).Append(SapPackageSet(t))
}

func azureEapPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"rhui-azure-rhel8",
		},
		Exclude: []string{
			"firewalld",
		},
	}.Append(azureCommonPackageSet(t))
}

// PARTITION TABLES

var azureRhuiBasePartitionTables = distro.BasePartitionTableMap{
	arch.ARCH_X86_64.String(): disk.PartitionTable{
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
					Type:         "xfs",
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
								Type:         "xfs",
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
								Type:         "xfs",
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
								Type:         "xfs",
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
								Type:         "xfs",
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
	arch.ARCH_AARCH64.String(): disk.PartitionTable{
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
							Size: 1 * common.GibiByte,
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
							Size: 2 * common.GibiByte,
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
							Size: 2 * common.GibiByte,
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
							Size: 10 * common.GibiByte,
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
							Size: 10 * common.GibiByte,
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

var defaultAzureImageConfig = &distro.ImageConfig{
	Timezone: common.ToPtr("Etc/UTC"),
	Locale:   common.ToPtr("en_US.UTF-8"),
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
		"nm-cloud-setup.service",
		"nm-cloud-setup.timer",
		"sshd",
		"systemd-resolved",
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
	DefaultTarget: common.ToPtr("multi-user.target"),
}

// Diff of the default Image Config compare to the `defaultAzureImageConfig`
var defaultAzureByosImageConfig = &distro.ImageConfig{
	GPGKeyFiles: []string{
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
	},
	RHSMConfig: map[subscription.RHSMStatus]*osbuild.RHSMStageOptions{
		subscription.RHSMConfigNoSubscription: {
			SubMan: &osbuild.RHSMStageOptionsSubMan{
				Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
					AutoRegistration: common.ToPtr(true),
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
		subscription.RHSMConfigWithSubscription: {
			SubMan: &osbuild.RHSMStageOptionsSubMan{
				Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
					AutoRegistration: common.ToPtr(true),
				},
				// do not disable the redhat.repo management if the user
				// explicitly request the system to be subscribed
			},
		},
	},
}

// Diff of the default Image Config compare to the `defaultAzureImageConfig`
var defaultAzureRhuiImageConfig = &distro.ImageConfig{
	GPGKeyFiles: []string{
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-microsoft-azure-release",
		"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
	},
	RHSMConfig: map[subscription.RHSMStatus]*osbuild.RHSMStageOptions{
		subscription.RHSMConfigNoSubscription: {
			DnfPlugins: &osbuild.RHSMStageOptionsDnfPlugins{
				SubscriptionManager: &osbuild.RHSMStageOptionsDnfPlugin{
					Enabled: false,
				},
			},
			SubMan: &osbuild.RHSMStageOptionsSubMan{
				Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
					AutoRegistration: common.ToPtr(true),
				},
				Rhsm: &osbuild.SubManConfigRHSMSection{
					ManageRepos: common.ToPtr(false),
				},
			},
		},
		subscription.RHSMConfigWithSubscription: {
			SubMan: &osbuild.RHSMStageOptionsSubMan{
				Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
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

func sapAzureImageConfig(rd distribution) *distro.ImageConfig {
	return sapImageConfig(rd).InheritFrom(defaultVhdImageConfig())
}
