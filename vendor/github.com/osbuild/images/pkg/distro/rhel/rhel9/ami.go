package rhel9

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
)

// TODO: move these to the EC2 environment
const amiKernelOptions = "console=tty0 console=ttyS0,115200n8 net.ifnames=0 nvme_core.io_timeout=4294967295"

// default EC2 images config (common for all architectures)
func baseEc2ImageConfig() *distro.ImageConfig {
	return &distro.ImageConfig{
		Locale:   common.ToPtr("en_US.UTF-8"),
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
			"tuned",
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
			{
				Filename: "blacklist-amdgpu.conf",
				Commands: osbuild.ModprobeConfigCmdList{
					osbuild.NewModprobeConfigCmdBlacklist("amdgpu"),
				},
			},
		},
		// COMPOSER-1807
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
						Environment: []osbuild.EnvironmentVariable{{Key: "NM_CLOUD_SETUP_EC2", Value: "yes"}},
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

func defaultEc2ImageConfig(osVersion string, rhsm bool) *distro.ImageConfig {
	ic := baseEc2ImageConfig()
	if rhsm && common.VersionLessThan(osVersion, "9.1") {
		ic = appendRHSM(ic)
		// Disable RHSM redhat.repo management
		rhsmConf := ic.RHSMConfig[subscription.RHSMConfigNoSubscription]
		rhsmConf.SubMan.Rhsm = subscription.SubManRHSMConfig{ManageRepos: common.ToPtr(false)}
		ic.RHSMConfig[subscription.RHSMConfigNoSubscription] = rhsmConf
	}
	return ic
}

func defaultEc2ImageConfigX86_64(osVersion string, rhsm bool) *distro.ImageConfig {
	ic := defaultEc2ImageConfig(osVersion, rhsm)
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

// common ec2 image build package set
func ec2BuildPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	return distroBuildPackageSet(t).Append(
		rpmmd.PackageSet{
			Include: []string{
				"python3-pyyaml",
			},
		})
}

func ec2CommonPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	ps := rpmmd.PackageSet{
		Include: []string{
			"@core",
			"authselect-compat",
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"dhcp-client",
			"yum-utils",
			"dracut-config-generic",
			"gdisk",
			"grub2",
			"langpacks-en",
			"NetworkManager-cloud-setup",
			"redhat-release",
			"redhat-release-eula",
			"rsync",
			"tuned",
			"tar",
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
			// RHBZ#2064087
			"dracut-config-rescue",
			// RHBZ#2075815
			"qemu-guest-agent",
		},
	}.Append(distroSpecificPackageSet(t))

	return ps
}

// common rhel ec2 RHUI image package set
func rhelEc2CommonPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	ps := ec2CommonPackageSet(t)
	// Include "redhat-cloud-client-configuration" on 9.1+ (COMPOSER-1805)
	if common.VersionGreaterThanOrEqual(t.Arch().Distro().OsVersion(), "9.1") {
		ps.Include = append(ps.Include, "redhat-cloud-client-configuration")
	}
	return ps
}

// rhel-ec2 image package set
func rhelEc2PackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	ec2PackageSet := rhelEc2CommonPackageSet(t)
	ec2PackageSet = ec2PackageSet.Append(rpmmd.PackageSet{
		Include: []string{
			"rh-amazon-rhui-client",
		},
		Exclude: []string{
			"alsa-lib",
		},
	})
	return ec2PackageSet
}

// rhel-ha-ec2 image package set
func rhelEc2HaPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	ec2HaPackageSet := rhelEc2CommonPackageSet(t)
	ec2HaPackageSet = ec2HaPackageSet.Append(rpmmd.PackageSet{
		Include: []string{
			"fence-agents-all",
			"pacemaker",
			"pcs",
			"rh-amazon-rhui-client-ha",
		},
		Exclude: []string{
			"alsa-lib",
		},
	})
	return ec2HaPackageSet
}

// rhel-sap-ec2 image package set
// Includes the common ec2 package set, the common SAP packages, and
// the amazon rhui sap package
func rhelEc2SapPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{
		Include: []string{
			"rh-amazon-rhui-client-sap-bundle-e4s",
			"libcanberra-gtk2",
		},
		Exclude: []string{
			// COMPOSER-1829
			"firewalld",
		},
	}.Append(rhelEc2CommonPackageSet(t)).Append(SapPackageSet(t))
}

func mkEc2ImgTypeX86_64(osVersion string, rhsm bool) *rhel.ImageType {
	it := rhel.NewImageType(
		"ec2",
		"image.raw.xz",
		"application/xz",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: rhelEc2PackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.KernelOptions = amiKernelOptions
	it.Bootable = true
	it.DefaultSize = 10 * common.GibiByte
	it.DefaultImageConfig = defaultEc2ImageConfigX86_64(osVersion, rhsm)
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkAMIImgTypeX86_64() *rhel.ImageType {
	it := rhel.NewImageType(
		"ami",
		"image.raw",
		"application/octet-stream",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: ec2CommonPackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image"},
		[]string{"image"},
	)

	it.KernelOptions = amiKernelOptions
	it.Bootable = true
	it.DefaultSize = 10 * common.GibiByte
	it.DefaultImageConfig = defaultAMIImageConfigX86_64()
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkEC2SapImgTypeX86_64(osVersion string, rhsm bool) *rhel.ImageType {
	it := rhel.NewImageType(
		"ec2-sap",
		"image.raw.xz",
		"application/xz",
		map[string]rhel.PackageSetFunc{
			rhel.BuildPkgsKey: ec2BuildPackageSet,
			rhel.OSPkgsKey:    rhelEc2SapPackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.KernelOptions = "console=ttyS0,115200n8 console=tty0 net.ifnames=0 nvme_core.io_timeout=4294967295 processor.max_cstate=1 intel_idle.max_cstate=1"
	it.Bootable = true
	it.DefaultSize = 10 * common.GibiByte
	it.DefaultImageConfig = sapImageConfig(osVersion).InheritFrom(defaultEc2ImageConfigX86_64(osVersion, rhsm))
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkEc2HaImgTypeX86_64(osVersion string, rhsm bool) *rhel.ImageType {
	it := rhel.NewImageType(
		"ec2-ha",
		"image.raw.xz",
		"application/xz",
		map[string]rhel.PackageSetFunc{
			rhel.BuildPkgsKey: ec2BuildPackageSet,
			rhel.OSPkgsKey:    rhelEc2HaPackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.KernelOptions = amiKernelOptions
	it.Bootable = true
	it.DefaultSize = 10 * common.GibiByte
	it.DefaultImageConfig = defaultEc2ImageConfigX86_64(osVersion, rhsm)
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkAMIImgTypeAarch64() *rhel.ImageType {
	it := rhel.NewImageType(
		"ami",
		"image.raw",
		"application/octet-stream",
		map[string]rhel.PackageSetFunc{
			rhel.BuildPkgsKey: ec2BuildPackageSet,
			rhel.OSPkgsKey:    ec2CommonPackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image"},
		[]string{"image"},
	)

	it.KernelOptions = "console=ttyS0,115200n8 console=tty0 net.ifnames=0 nvme_core.io_timeout=4294967295 iommu.strict=0"
	it.Bootable = true
	it.DefaultSize = 10 * common.GibiByte
	it.DefaultImageConfig = defaultAMIImageConfig()
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkEC2ImgTypeAarch64(osVersion string, rhsm bool) *rhel.ImageType {
	it := rhel.NewImageType(
		"ec2",
		"image.raw.xz",
		"application/xz",
		map[string]rhel.PackageSetFunc{
			rhel.BuildPkgsKey: ec2BuildPackageSet,
			rhel.OSPkgsKey:    rhelEc2PackageSet,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.KernelOptions = "console=ttyS0,115200n8 console=tty0 net.ifnames=0 nvme_core.io_timeout=4294967295 iommu.strict=0"
	it.Bootable = true
	it.DefaultSize = 10 * common.GibiByte
	it.DefaultImageConfig = defaultEc2ImageConfig(osVersion, rhsm)
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// Add RHSM config options to ImageConfig.
// Used for RHEL distros.
func appendRHSM(ic *distro.ImageConfig) *distro.ImageConfig {
	rhsm := &distro.ImageConfig{
		RHSMConfig: map[subscription.RHSMStatus]*subscription.RHSMConfig{
			subscription.RHSMConfigNoSubscription: {
				// RHBZ#1932802
				SubMan: subscription.SubManConfig{
					Rhsmcertd: subscription.SubManRHSMCertdConfig{
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
