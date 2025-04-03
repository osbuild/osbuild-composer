package rhel10

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
func mkAzureImgType(rd *rhel.Distribution) *rhel.ImageType {
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

	it.KernelOptions = defaultAzureKernelOptions()
	it.Bootable = true
	it.DefaultSize = 4 * datasizes.GibiByte
	it.DefaultImageConfig = defaultAzureImageConfig(rd)
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// Azure RHEL-internal image type
func mkAzureInternalImgType(rd *rhel.Distribution) *rhel.ImageType {
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
	it.KernelOptions = defaultAzureKernelOptions()
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.DefaultImageConfig = defaultAzureImageConfig(rd)
	it.BasePartitionTables = azureInternalBasePartitionTables

	return it
}

func mkAzureSapInternalImgType(rd *rhel.Distribution) *rhel.ImageType {
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
	it.KernelOptions = defaultAzureKernelOptions()
	it.Bootable = true
	it.DefaultSize = 64 * datasizes.GibiByte
	it.DefaultImageConfig = sapAzureImageConfig(rd)
	it.BasePartitionTables = azureInternalBasePartitionTables

	return it
}

// PARTITION TABLES
func azureInternalBasePartitionTables(t *rhel.ImageType) (disk.PartitionTable, bool) {
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
				// NB: we currently don't support /boot on LVM
				{
					Size: 1 * datasizes.GibiByte,
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
				// NB: we currently don't support /boot on LVM
				{
					Size: 1 * datasizes.GibiByte,
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
func defaultAzureKernelOptions() []string {
	return []string{"ro", "loglevel=3", "console=tty1", "console=ttyS0", "earlyprintk=ttyS0", "rootdelay=300"}
}

// based on https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/9/html/deploying_rhel_9_on_microsoft_azure/assembly_deploying-a-rhel-image-as-a-virtual-machine-on-microsoft-azure_cloud-content-azure#making-configuration-changes_configure-the-image-azure
func defaultAzureImageConfig(rd *rhel.Distribution) *distro.ImageConfig {
	ic := &distro.ImageConfig{
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
		SystemdUnit: []*osbuild.SystemdUnitStageOptions{
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
	}

	return ic
}

func sapAzureImageConfig(rd *rhel.Distribution) *distro.ImageConfig {
	return sapImageConfig(rd.OsVersion()).InheritFrom(defaultAzureImageConfig(rd))
}
