package rhel10

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
)

// TODO: move these to the EC2 environment

func amiKernelOptions() []string {
	return []string{"console=tty0", "console=ttyS0,115200n8", "nvme_core.io_timeout=4294967295"}
}

func amiAarch64KernelOptions() []string {
	return append(amiKernelOptions(), "iommu.strict=0")
}

func amiSapKernelOptions() []string {
	return append(amiKernelOptions(), []string{"processor.max_cstate=1", "intel_idle.max_cstate=1"}...)
}

func mkAMIImgTypeX86_64() *rhel.ImageType {
	it := rhel.NewImageType(
		"ami",
		"image.raw",
		"application/octet-stream",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image"},
		[]string{"image"},
	)

	it.KernelOptions = amiKernelOptions()
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = defaultEc2ImageConfigX86_64()
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkAMIImgTypeAarch64() *rhel.ImageType {
	it := rhel.NewImageType(
		"ami",
		"image.raw",
		"application/octet-stream",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image"},
		[]string{"image"},
	)

	it.KernelOptions = amiAarch64KernelOptions()
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = defaultEc2ImageConfig()
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// RHEL internal-only x86_64 EC2 image type
func mkEc2ImgTypeX86_64() *rhel.ImageType {
	it := rhel.NewImageType(
		"ec2",
		"image.raw.xz",
		"application/xz",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.KernelOptions = amiKernelOptions()
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = defaultEc2ImageConfigX86_64()
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// RHEL internal-only aarch64 EC2 image type
func mkEC2ImgTypeAarch64() *rhel.ImageType {
	it := rhel.NewImageType(
		"ec2",
		"image.raw.xz",
		"application/xz",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.KernelOptions = amiAarch64KernelOptions()
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = defaultEc2ImageConfig()
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// RHEL internal-only x86_64 EC2 HA image type
func mkEc2HaImgTypeX86_64() *rhel.ImageType {
	it := rhel.NewImageType(
		"ec2-ha",
		"image.raw.xz",
		"application/xz",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.KernelOptions = amiKernelOptions()
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = defaultEc2ImageConfigX86_64()
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkEC2SapImgTypeX86_64(osVersion string) *rhel.ImageType {
	it := rhel.NewImageType(
		"ec2-sap",
		"image.raw.xz",
		"application/xz",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.KernelOptions = amiSapKernelOptions()
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = sapImageConfig(osVersion).InheritFrom(defaultEc2ImageConfigX86_64())
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// IMAGE CONFIG

// default EC2 images config (common for all architectures)
func defaultEc2ImageConfig() *distro.ImageConfig {
	return &distro.ImageConfig{
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
		DefaultTarget:       common.ToPtr("multi-user.target"),
		UpdateDefaultKernel: common.ToPtr(true),
		DefaultKernel:       common.ToPtr("kernel"),
		Sysconfig: &distro.Sysconfig{
			Networking: true,
			NoZeroConf: true,
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
			// https://issues.redhat.com/browse/RHEL-71926
			{
				Filename: "blacklist-i2c_piix4.conf",
				Commands: osbuild.ModprobeConfigCmdList{
					osbuild.NewModprobeConfigCmdBlacklist("i2c_piix4"),
				},
			},
		},
		SystemdDropin: []*osbuild.SystemdUnitStageOptions{
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

func defaultEc2ImageConfigX86_64() *distro.ImageConfig {
	ic := defaultEc2ImageConfig()
	return appendEC2DracutX86_64(ic)
}
