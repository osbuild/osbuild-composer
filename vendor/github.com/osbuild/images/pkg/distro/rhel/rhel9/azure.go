package rhel9

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
)

// Azure image type
func mkAzureImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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

	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.DefaultImageConfig = defaultAzureImageConfig(rd)
	it.DefaultImageConfig.KernelOptions = defaultAzureKernelOptions(rd, a)
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// Azure RHEL-internal image type
func mkAzureInternalImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.DefaultImageConfig = defaultAzureImageConfig(rd)
	it.DefaultImageConfig.KernelOptions = defaultAzureKernelOptions(rd, a)
	it.BasePartitionTables = azureInternalBasePartitionTables

	return it
}

func mkAzureSapInternalImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
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
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.DefaultImageConfig = sapAzureImageConfig(rd)
	it.DefaultImageConfig.KernelOptions = defaultAzureKernelOptions(rd, a)
	it.BasePartitionTables = azureInternalBasePartitionTables

	return it
}

// PARTITION TABLES
func azureInternalBasePartitionTables(t *rhel.ImageType) (disk.PartitionTable, bool) {
	var bootSize uint64
	switch {
	case common.VersionLessThan(t.Arch().Distro().OsVersion(), "9.3") && t.IsRHEL():
		// RHEL <= 9.2 had only 500 MiB /boot
		bootSize = 500 * datasizes.MebiByte
	case common.VersionLessThan(t.Arch().Distro().OsVersion(), "9.4") && t.IsRHEL():
		// RHEL 9.3 had 600 MiB /boot, see RHEL-7999
		bootSize = 600 * datasizes.MebiByte
	default:
		// RHEL >= 9.4 needs to have even a bigger /boot, see COMPOSER-2155
		bootSize = 1 * datasizes.GibiByte
	}

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
					Size: bootSize,
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
					Size: bootSize,
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

// IMAGE CONFIG

// use loglevel=3 as described in the RHEL documentation and used in existing RHEL images built by MSFT
func defaultAzureKernelOptions(rd *rhel.Distribution, a arch.Arch) []string {
	kargs := []string{"ro", "loglevel=3"}
	switch a {
	case arch.ARCH_AARCH64:
		kargs = append(kargs, "console=ttyAMA0")
	case arch.ARCH_X86_64:
		kargs = append(kargs, "console=tty1", "console=ttyS0", "earlyprintk=ttyS0", "rootdelay=300")
	}
	if rd.IsRHEL() && common.VersionGreaterThanOrEqual(rd.OsVersion(), "9.6") {
		kargs = append(kargs, "nvme_core.io_timeout=240")
	}
	return kargs
}

// based on https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/9/html/deploying_rhel_9_on_microsoft_azure/assembly_deploying-a-rhel-image-as-a-virtual-machine-on-microsoft-azure_cloud-content-azure#making-configuration-changes_configure-the-image-azure
func defaultAzureImageConfig(rd *rhel.Distribution) *distro.ImageConfig {
	ic := &distro.ImageConfig{
		Timezone: common.ToPtr("Etc/UTC"),
		Locale:   common.ToPtr("en_US.UTF-8"),
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
			X11Keymap: &osbuild.X11KeymapOptions{
				Layouts: []string{"us"},
			},
		},
		UpdateDefaultKernel: common.ToPtr(true),
		DefaultKernel:       common.ToPtr("kernel-core"),
		Sysconfig: &distro.Sysconfig{
			Networking: true,
			NoZeroConf: true,
		},
		EnabledServices: []string{
			"firewalld",
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

	if rd.IsRHEL() {
		ic.GPGKeyFiles = append(ic.GPGKeyFiles, "/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release")
		if common.VersionGreaterThanOrEqual(rd.OsVersion(), "9.6") {
			ic.Modprobe = append(
				ic.Modprobe,
				&osbuild.ModprobeStageOptions{
					Filename: "blacklist-intel_uncore.conf",
					Commands: osbuild.ModprobeConfigCmdList{
						osbuild.NewModprobeConfigCmdBlacklist("intel_uncore"),
					},
				},
				&osbuild.ModprobeStageOptions{
					Filename: "blacklist-acpi_cpufreq.conf",
					Commands: osbuild.ModprobeConfigCmdList{
						osbuild.NewModprobeConfigCmdBlacklist("acpi_cpufreq"),
					},
				},
			)

			ic.WAAgentConfig.Config.ProvisioningUseCloudInit = common.ToPtr(true)
			ic.WAAgentConfig.Config.ProvisioningEnabled = common.ToPtr(false)

			ic.TimeSynchronization = &osbuild.ChronyStageOptions{
				Refclocks: []osbuild.ChronyConfigRefclock{
					{
						Driver: osbuild.NewChronyDriverPHC("/dev/ptp_hyperv"),
						Poll:   common.ToPtr(3),
						Dpoll:  common.ToPtr(-2),
						Offset: common.ToPtr(0.0),
					},
				},
			}

			datalossWarningScript, datalossSystemdUnit, err := rhel.CreateAzureDatalossWarningScriptAndUnit()
			if err != nil {
				panic(err)
			}
			ic.Files = append(ic.Files, datalossWarningScript)
			ic.SystemdUnit = append(ic.SystemdUnit, datalossSystemdUnit)
			ic.EnabledServices = append(ic.EnabledServices, datalossSystemdUnit.Filename)
			ic.NetworkManager = &osbuild.NMConfStageOptions{
				Path: "/etc/NetworkManager/conf.d/99-azure-unmanaged-devices.conf",
				Settings: osbuild.NMConfStageSettings{
					Keyfile: &osbuild.NMConfSettingsKeyfile{
						UnmanagedDevices: []string{
							"driver:mlx4_core",
							"driver:mlx5_core",
						},
					},
				},
			}
		}
	}

	return ic
}

func sapAzureImageConfig(rd *rhel.Distribution) *distro.ImageConfig {
	return sapImageConfig(rd.OsVersion()).InheritFrom(defaultAzureImageConfig(rd))
}
