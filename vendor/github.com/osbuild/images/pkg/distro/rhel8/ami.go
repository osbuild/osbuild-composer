package rhel8

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/subscription"
)

func amiImgTypeX86_64(rd distribution) imageType {
	it := imageType{
		name:     "ami",
		filename: "image.raw",
		mimeType: "application/octet-stream",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: ec2CommonPackageSet,
		},
		defaultImageConfig:  defaultAMIImageConfigX86_64(),
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto",
		bootable:            true,
		defaultSize:         10 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image"},
		exports:             []string{"image"},
		basePartitionTables: getEc2PartitionTables(rd.osVersion, rd.isRHEL()),
	}

	return it
}

func ec2ImgTypeX86_64(rd distribution) imageType {
	it := imageType{
		name:        "ec2",
		filename:    "image.raw.xz",
		mimeType:    "application/xz",
		compression: "xz",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: rhelEc2PackageSet,
		},
		defaultImageConfig:  defaultEc2ImageConfigX86_64(rd),
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto",
		bootable:            true,
		defaultSize:         10 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "xz"},
		exports:             []string{"xz"},
		basePartitionTables: getEc2PartitionTables(rd.osVersion, rd.isRHEL()),
	}
	return it
}

func ec2HaImgTypeX86_64(rd distribution) imageType {
	it := imageType{
		name:        "ec2-ha",
		filename:    "image.raw.xz",
		mimeType:    "application/xz",
		compression: "xz",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: rhelEc2HaPackageSet,
		},
		defaultImageConfig:  defaultEc2ImageConfigX86_64(rd),
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto",
		bootable:            true,
		defaultSize:         10 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "xz"},
		exports:             []string{"xz"},
		basePartitionTables: getEc2PartitionTables(rd.osVersion, rd.isRHEL()),
	}
	return it
}

func amiImgTypeAarch64(rd distribution) imageType {
	it := imageType{
		name:     "ami",
		filename: "image.raw",
		mimeType: "application/octet-stream",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: ec2CommonPackageSet,
		},
		defaultImageConfig:  defaultAMIImageConfig(),
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 iommu.strict=0 crashkernel=auto",
		bootable:            true,
		defaultSize:         10 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image"},
		exports:             []string{"image"},
		basePartitionTables: getEc2PartitionTables(rd.osVersion, rd.isRHEL()),
	}
	return it
}

func ec2ImgTypeAarch64(rd distribution) imageType {
	it := imageType{
		name:        "ec2",
		filename:    "image.raw.xz",
		mimeType:    "application/xz",
		compression: "xz",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: rhelEc2PackageSet,
		},
		defaultImageConfig:  defaultEc2ImageConfig(rd),
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 iommu.strict=0 crashkernel=auto",
		bootable:            true,
		defaultSize:         10 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "xz"},
		exports:             []string{"xz"},
		basePartitionTables: getEc2PartitionTables(rd.osVersion, rd.isRHEL()),
	}
	return it
}

func ec2SapImgTypeX86_64(rd distribution) imageType {
	it := imageType{
		name:        "ec2-sap",
		filename:    "image.raw.xz",
		mimeType:    "application/xz",
		compression: "xz",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: rhelEc2SapPackageSet,
		},
		defaultImageConfig:  defaultEc2SapImageConfigX86_64(rd),
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto processor.max_cstate=1 intel_idle.max_cstate=1",
		bootable:            true,
		defaultSize:         10 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "xz"},
		exports:             []string{"xz"},
		basePartitionTables: getEc2PartitionTables(rd.osVersion, rd.isRHEL()),
	}
	return it
}

// default EC2 images config (common for all architectures)
func baseEc2ImageConfig() *distro.ImageConfig {
	return &distro.ImageConfig{
		Timezone: common.ToPtr("UTC"),
		TimeSynchronization: &osbuild.ChronyStageOptions{
			Servers: []osbuild.ChronyConfigServer{
				{
					Hostname: "169.254.169.123",
					Prefer:   common.ToPtr(true),
					Iburst:   common.ToPtr(true),
					Minpoll:  common.ToPtr(4),
					Maxpoll:  common.ToPtr(4),
				},
			},
			// empty string will remove any occurrences of the option from the configuration
			LeapsecTz: common.ToPtr(""),
		},
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
			X11Keymap: &osbuild.X11KeymapOptions{
				Layouts: []string{"us"},
			},
		},
		EnabledServices: []string{
			"sshd",
			"NetworkManager",
			"nm-cloud-setup.service",
			"nm-cloud-setup.timer",
			"cloud-init",
			"cloud-init-local",
			"cloud-config",
			"cloud-final",
			"reboot.target",
		},
		DefaultTarget: common.ToPtr("multi-user.target"),
		Sysconfig: []*osbuild.SysconfigStageOptions{
			{
				Kernel: &osbuild.SysconfigKernelOptions{
					UpdateDefault: true,
					DefaultKernel: "kernel",
				},
				Network: &osbuild.SysconfigNetworkOptions{
					Networking: true,
					NoZeroConf: true,
				},
				NetworkScripts: &osbuild.NetworkScriptsOptions{
					IfcfgFiles: map[string]osbuild.IfcfgFile{
						"eth0": {
							Device:    "eth0",
							Bootproto: osbuild.IfcfgBootprotoDHCP,
							OnBoot:    common.ToPtr(true),
							Type:      osbuild.IfcfgTypeEthernet,
							UserCtl:   common.ToPtr(true),
							PeerDNS:   common.ToPtr(true),
							IPv6Init:  common.ToPtr(false),
						},
					},
				},
			},
		},
		SystemdLogind: []*osbuild.SystemdLogindStageOptions{
			{
				Filename: "00-getty-fixes.conf",
				Config: osbuild.SystemdLogindConfigDropin{
					Login: osbuild.SystemdLogindConfigLoginSection{
						NAutoVTs: common.ToPtr(0),
					},
				},
			},
		},
		CloudInit: []*osbuild.CloudInitStageOptions{
			{
				Filename: "00-rhel-default-user.cfg",
				Config: osbuild.CloudInitConfigFile{
					SystemInfo: &osbuild.CloudInitConfigSystemInfo{
						DefaultUser: &osbuild.CloudInitConfigDefaultUser{
							Name: "ec2-user",
						},
					},
				},
			},
		},
		Modprobe: []*osbuild.ModprobeStageOptions{
			{
				Filename: "blacklist-nouveau.conf",
				Commands: osbuild.ModprobeConfigCmdList{
					osbuild.NewModprobeConfigCmdBlacklist("nouveau"),
				},
			},
			// COMPOSER-1807
			{
				Filename: "blacklist-amdgpu.conf",
				Commands: osbuild.ModprobeConfigCmdList{
					osbuild.NewModprobeConfigCmdBlacklist("amdgpu"),
				},
			},
		},
		DracutConf: []*osbuild.DracutConfStageOptions{
			{
				Filename: "sgdisk.conf",
				Config: osbuild.DracutConfigFile{
					Install: []string{"sgdisk"},
				},
			},
		},
		SystemdUnit: []*osbuild.SystemdUnitStageOptions{
			// RHBZ#1822863
			{
				Unit:   "nm-cloud-setup.service",
				Dropin: "10-rh-enable-for-ec2.conf",
				Config: osbuild.SystemdServiceUnitDropin{
					Service: &osbuild.SystemdUnitServiceSection{
						Environment: "NM_CLOUD_SETUP_EC2=yes",
					},
				},
			},
		},
		Authselect: &osbuild.AuthselectStageOptions{
			Profile: "sssd",
		},
		SshdConfig: &osbuild.SshdConfigStageOptions{
			Config: osbuild.SshdConfigConfig{
				PasswordAuthentication: common.ToPtr(false),
			},
		},
	}
}

func defaultEc2ImageConfig(rd distribution) *distro.ImageConfig {
	ic := baseEc2ImageConfig()
	// The RHSM configuration should not be applied since 8.7, but it is instead done by installing the
	// redhat-cloud-client-configuration package. See COMPOSER-1804 for more information.
	if rd.isRHEL() && common.VersionLessThan(rd.osVersion, "8.7") {
		ic = appendRHSM(ic)
		// Disable RHSM redhat.repo management
		rhsmConf := ic.RHSMConfig[subscription.RHSMConfigNoSubscription]
		rhsmConf.SubMan.Rhsm = &osbuild.SubManConfigRHSMSection{ManageRepos: common.ToPtr(false)}
		ic.RHSMConfig[subscription.RHSMConfigNoSubscription] = rhsmConf
	}

	return ic
}

func defaultEc2ImageConfigX86_64(rd distribution) *distro.ImageConfig {
	ic := defaultEc2ImageConfig(rd)
	return appendEC2DracutX86_64(ic)
}

// Default AMI (custom image built by users) images config.
// The configuration does not touch the RHSM configuration at all.
// https://issues.redhat.com/browse/COMPOSER-2157
func defaultAMIImageConfig() *distro.ImageConfig {
	return baseEc2ImageConfig()
}

// Default AMI x86_64 (custom image built by users) images config.
// The configuration does not touch the RHSM configuration at all.
// https://issues.redhat.com/browse/COMPOSER-2157
func defaultAMIImageConfigX86_64() *distro.ImageConfig {
	ic := defaultAMIImageConfig()
	return appendEC2DracutX86_64(ic)
}

func defaultEc2SapImageConfigX86_64(rd distribution) *distro.ImageConfig {
	// default EC2-SAP image config (x86_64)
	return sapImageConfig(rd).InheritFrom(defaultEc2ImageConfigX86_64(rd))
}

// common package set for RHEL (BYOS/RHUI) and CentOS Stream images
func ec2CommonPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"@core",
			"authselect-compat",
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"dhcp-client",
			"dracut-config-generic",
			"dracut-norescue",
			"gdisk",
			"grub2",
			"langpacks-en",
			"NetworkManager",
			"NetworkManager-cloud-setup",
			"redhat-release",
			"redhat-release-eula",
			"rsync",
			"tar",
			"yum-utils",
		},
		Exclude: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-tools-firmware",
			"biosdevname",
			"firewalld",
			"iprutils",
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
			"plymouth",
			// RHBZ#2075815
			"qemu-guest-agent",
		},
	}.Append(distroSpecificPackageSet(t))
}

// common rhel ec2 RHUI image package set
func rhelEc2CommonPackageSet(t *imageType) rpmmd.PackageSet {
	ps := ec2CommonPackageSet(t)
	// Include "redhat-cloud-client-configuration" on 8.7+ (COMPOSER-1804)
	if common.VersionGreaterThanOrEqual(t.arch.distro.osVersion, "8.7") {
		ps.Include = append(ps.Include, "redhat-cloud-client-configuration")
	}
	return ps
}

// rhel-ec2 image package set
func rhelEc2PackageSet(t *imageType) rpmmd.PackageSet {
	ec2PackageSet := rhelEc2CommonPackageSet(t)
	ec2PackageSet.Include = append(ec2PackageSet.Include, "rh-amazon-rhui-client")
	ec2PackageSet.Exclude = append(ec2PackageSet.Exclude, "alsa-lib")
	return ec2PackageSet
}

// rhel-ha-ec2 image package set
func rhelEc2HaPackageSet(t *imageType) rpmmd.PackageSet {
	ec2HaPackageSet := rhelEc2CommonPackageSet(t)
	ec2HaPackageSet.Include = append(ec2HaPackageSet.Include,
		"fence-agents-all",
		"pacemaker",
		"pcs",
		"rh-amazon-rhui-client-ha",
	)
	ec2HaPackageSet.Exclude = append(ec2HaPackageSet.Exclude, "alsa-lib")
	return ec2HaPackageSet
}

// rhel-sap-ec2 image package set
// Includes the common ec2 package set, the common SAP packages, and
// the amazon rhui sap package
func rhelEc2SapPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"rh-amazon-rhui-client-sap-bundle-e4s",
		},
	}.Append(rhelEc2CommonPackageSet(t)).Append(SapPackageSet(t))
}

// Add RHSM config options to ImageConfig.
// Used for RHEL distros.
func appendRHSM(ic *distro.ImageConfig) *distro.ImageConfig {
	rhsm := &distro.ImageConfig{
		RHSMConfig: map[subscription.RHSMStatus]*osbuild.RHSMStageOptions{
			subscription.RHSMConfigNoSubscription: {
				// RHBZ#1932802
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.ToPtr(true),
					},
					// Don't disable RHSM redhat.repo management on the AMI
					// image, which is BYOS and does not use RHUI for content.
					// Otherwise subscribing the system manually after booting
					// it would result in empty redhat.repo. Without RHUI, such
					// system would have no way to get Red Hat content, but
					// enable the repo management manually, which would be very
					// confusing.
				},
			},
			subscription.RHSMConfigWithSubscription: {
				// RHBZ#1932802
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
	return rhsm.InheritFrom(ic)
}

func appendEC2DracutX86_64(ic *distro.ImageConfig) *distro.ImageConfig {
	ic.DracutConf = append(ic.DracutConf,
		&osbuild.DracutConfStageOptions{
			Filename: "ec2.conf",
			Config: osbuild.DracutConfigFile{
				AddDrivers: []string{
					"nvme",
					"xen-blkfront",
				},
			},
		})
	return ic
}
