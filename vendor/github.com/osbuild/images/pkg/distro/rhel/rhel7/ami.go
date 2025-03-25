package rhel7

import (
	"os"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
)

func ec2KernelOptions() []string {
	return []string{"ro", "console=tty0", "console=ttyS0,115200n8", "net.ifnames=0", "rd.blacklist=nouveau", "nvme_core.io_timeout=4294967295", "crashkernel=auto", "LANG=en_US.UTF-8"}
}

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

	// all RHEL 7 images should use sgdisk
	it.DiskImagePartTool = common.ToPtr(osbuild.PTSgdisk)

	it.Compression = "xz"
	it.DefaultImageConfig = ec2ImageConfig()
	it.KernelOptions = ec2KernelOptions()
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.BasePartitionTables = ec2PartitionTables

	return it
}

// default EC2 images config (common for all architectures)
func ec2ImageConfig() *distro.ImageConfig {

	// systemd-firstboot on el7 does not support --keymap option
	vconsoleFile, err := fsnode.NewFile("/etc/vconsole.conf", nil, nil, nil, []byte("FONT=latarcyrheb-sun16\nKEYMAP=us\n"))
	if err != nil {
		panic(err)
	}

	// This is needed to disable predictable network interface names.
	// The org.osbuild.udev.rules stage can't create empty files.
	udevNetNameSlotRulesFile, err := fsnode.NewFile("/etc/udev/rules.d/80-net-name-slot.rules", nil, nil, nil, []byte{})
	if err != nil {
		panic(err)
	}

	// While cloud-init does this automatically on first boot for the specified user,
	// this was in the original KS.
	ec2UserSudoers, err := fsnode.NewFile("/etc/sudoers.d/ec2-user", common.ToPtr(os.FileMode(0o440)), nil, nil, []byte("ec2-user\tALL=(ALL)\tNOPASSWD: ALL\n"))
	if err != nil {
		panic(err)
	}

	// The image built from the original KS has this file with this content.
	hostnameFile, err := fsnode.NewFile("/etc/hostname", nil, nil, nil, []byte("localhost.localdomain\n"))
	if err != nil {
		panic(err)
	}

	return &distro.ImageConfig{
		Timezone: common.ToPtr("UTC"),
		TimeSynchronization: &osbuild.ChronyStageOptions{
			Servers: []osbuild.ChronyConfigServer{
				{
					Hostname: "0.rhel.pool.ntp.org",
					Iburst:   common.ToPtr(true),
				},
				{
					Hostname: "1.rhel.pool.ntp.org",
					Iburst:   common.ToPtr(true),
				},
				{
					Hostname: "2.rhel.pool.ntp.org",
					Iburst:   common.ToPtr(true),
				},
				{
					Hostname: "3.rhel.pool.ntp.org",
					Iburst:   common.ToPtr(true),
				},
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
		EnabledServices: []string{
			"sshd",
			"rsyslog",
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
				Filename: "logind.conf",
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
			{
				Filename: "99-datasource.cfg",
				Config: osbuild.CloudInitConfigFile{
					DatasourceList: []string{
						"Ec2",
						"None",
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
		},
		DracutConf: []*osbuild.DracutConfStageOptions{
			{
				Filename: "sgdisk.conf",
				Config: osbuild.DracutConfigFile{
					Install: []string{"sgdisk"},
				},
			},
		},
		SshdConfig: &osbuild.SshdConfigStageOptions{
			Config: osbuild.SshdConfigConfig{
				PasswordAuthentication: common.ToPtr(false),
			},
		},
		Files: []*fsnode.File{
			vconsoleFile,
			udevNetNameSlotRulesFile,
			ec2UserSudoers,
			hostnameFile,
		},
		SELinuxForceRelabel: common.ToPtr(true),
	}
}

func ec2PartitionTables(t *rhel.ImageType) (disk.PartitionTable, bool) {
	switch t.Arch().Name() {
	case arch.ARCH_X86_64.String():
		return disk.PartitionTable{
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: disk.PT_GPT,
			Size: 10 * datasizes.GibiByte,
			Partitions: []disk.Partition{
				{
					Size:     1 * datasizes.MebiByte,
					Bootable: true,
					Type:     disk.BIOSBootPartitionGUID,
					UUID:     disk.BIOSBootPartitionUUID,
				},
				{
					Size: 6144 * datasizes.MebiByte,
					Type: disk.FilesystemDataGUID,
					UUID: disk.RootPartitionUUID,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						Label:        "root",
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		}, true

	default:
		return disk.PartitionTable{}, false
	}
}
