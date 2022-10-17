package rhel9

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/oscap"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
)

var (
	// rhel9 & cs9 share the same list
	// of allowed profiles so a single
	// allow list can be used
	oscapProfileAllowList = []oscap.Profile{
		oscap.AnssiBp28Enhanced,
		oscap.AnssiBp28High,
		oscap.AnssiBp28Intermediary,
		oscap.AnssiBp28Minimal,
		oscap.Cis,
		oscap.CisServerL1,
		oscap.CisWorkstationL1,
		oscap.CisWorkstationL2,
		oscap.Cui,
		oscap.E8,
		oscap.Hippa,
		oscap.IsmO,
		oscap.Ospp,
		oscap.PciDss,
		oscap.Stig,
		oscap.StigGui,
	}
)

type distribution struct {
	name               string
	product            string
	osVersion          string
	releaseVersion     string
	modulePlatformID   string
	vendor             string
	ostreeRefTmpl      string
	isolabelTmpl       string
	runner             runner.Runner
	arches             map[string]distro.Arch
	defaultImageConfig *distro.ImageConfig
}

// CentOS- and RHEL-based OS image configuration defaults
var defaultDistroImageConfig = &distro.ImageConfig{
	Timezone: common.StringToPtr("America/New_York"),
	Locale:   common.StringToPtr("C.UTF-8"),
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
		},
	},
}

func getCentOSDistro(version int) distribution {
	return distribution{
		name:               fmt.Sprintf("centos-%d", version),
		product:            "CentOS Stream",
		osVersion:          fmt.Sprintf("%d-stream", version),
		releaseVersion:     strconv.Itoa(version),
		modulePlatformID:   fmt.Sprintf("platform:el%d", version),
		vendor:             "centos",
		ostreeRefTmpl:      fmt.Sprintf("centos/%d/%%s/edge", version),
		isolabelTmpl:       fmt.Sprintf("CentOS-Stream-%d-BaseOS-%%s", version),
		runner:             &runner.CentOS{Version: uint64(version)},
		defaultImageConfig: defaultDistroImageConfig,
	}
}

func getRHELDistro(major, minor int) distribution {
	return distribution{
		name:               fmt.Sprintf("rhel-%d%d", major, minor),
		product:            "Red Hat Enterprise Linux",
		osVersion:          fmt.Sprintf("%d.%d", major, minor),
		releaseVersion:     strconv.Itoa(major),
		modulePlatformID:   fmt.Sprintf("platform:el%d", major),
		vendor:             "redhat",
		ostreeRefTmpl:      fmt.Sprintf("rhel/%d/%%s/edge", major),
		isolabelTmpl:       fmt.Sprintf("RHEL-%d-%d-0-BaseOS-%%s", major, minor),
		runner:             &runner.RHEL{Major: uint64(major), Minor: uint64(minor)},
		defaultImageConfig: defaultDistroImageConfig,
	}
}

func (d *distribution) Name() string {
	return d.name
}

func (d *distribution) Releasever() string {
	return d.releaseVersion
}

func (d *distribution) ModulePlatformID() string {
	return d.modulePlatformID
}

func (d *distribution) OSTreeRef() string {
	return d.ostreeRefTmpl
}

func (d *distribution) ListArches() []string {
	archNames := make([]string, 0, len(d.arches))
	for name := range d.arches {
		archNames = append(archNames, name)
	}
	sort.Strings(archNames)
	return archNames
}

func (d *distribution) GetArch(name string) (distro.Arch, error) {
	arch, exists := d.arches[name]
	if !exists {
		return nil, errors.New("invalid architecture: " + name)
	}
	return arch, nil
}

func (d *distribution) addArches(arches ...architecture) {
	if d.arches == nil {
		d.arches = map[string]distro.Arch{}
	}

	// Do not make copies of architectures, as opposed to image types,
	// because architecture definitions are not used by more than a single
	// distro definition.
	for idx := range arches {
		d.arches[arches[idx].name] = &arches[idx]
	}
}

func (d *distribution) isRHEL() bool {
	return strings.HasPrefix(d.name, "rhel")
}

func (d *distribution) getDefaultImageConfig() *distro.ImageConfig {
	return d.defaultImageConfig
}

func New() distro.Distro {
	return NewRHEL90()
}

func NewHostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	// NOTE: args are ignored - host distro constructors are deprecated
	return NewRHEL90()
}

func NewCentOS9() distro.Distro {
	return newDistro("centos", 9, 0)
}

func NewCentOS9HostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	// NOTE: args are ignored - host distro constructors are deprecated
	return NewCentOS9()
}

func NewRHEL90() distro.Distro {
	return newDistro("rhel", 9, 0)
}

func NewRHEL90HostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	// NOTE: args are ignored - host distro constructors are deprecated
	return NewRHEL90()
}

func NewRHEL91() distro.Distro {
	return newDistro("rhel", 9, 1)
}

func NewRHEL91HostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	// NOTE: args are ignored - host distro constructors are deprecated
	return NewRHEL91()
}

func NewRHEL92() distro.Distro {
	return newDistro("rhel", 9, 2)
}

func NewRHEL92HostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	// NOTE: args are ignored - host distro constructors are deprecated
	return NewRHEL92()
}

func newDistro(name string, major, minor int) distro.Distro {
	var rd distribution
	switch name {
	case "rhel":
		rd = getRHELDistro(major, minor)
	case "centos":
		rd = getCentOSDistro(major)
	default:
		panic(fmt.Sprintf("unknown distro name: %s", name))
	}

	// Architecture definitions
	x86_64 := architecture{
		name:     distro.X86_64ArchName,
		distro:   &rd,
		legacy:   "i386-pc",
		bootType: distro.HybridBootType,
	}

	aarch64 := architecture{
		name:     distro.Aarch64ArchName,
		distro:   &rd,
		bootType: distro.UEFIBootType,
	}

	ppc64le := architecture{
		distro:   &rd,
		name:     distro.Ppc64leArchName,
		legacy:   "powerpc-ieee1275",
		bootType: distro.LegacyBootType,
	}
	s390x := architecture{
		distro:   &rd,
		name:     distro.S390xArchName,
		bootType: distro.LegacyBootType,
	}

	// Shared Services
	edgeServices := []string{
		// TODO(runcom): move fdo-client-linuxapp.service to presets?
		"NetworkManager.service", "firewalld.service", "sshd.service", "fdo-client-linuxapp.service",
	}

	// Image Definitions
	edgeCommitImgType := imageType{
		name:        "edge-commit",
		nameAliases: []string{"rhel-edge-commit"},
		filename:    "commit.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: edgeBuildPackageSet,
			osPkgsKey:    edgeCommitPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: edgeServices,
		},
		rpmOstree:        true,
		pipelines:        edgeCommitPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "commit-archive"},
		exports:          []string{"commit-archive"},
	}

	edgeOCIImgType := imageType{
		name:        "edge-container",
		nameAliases: []string{"rhel-edge-container"},
		filename:    "container.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: edgeBuildPackageSet,
			osPkgsKey:    edgeCommitPackageSet,
			containerPkgsKey: func(t *imageType) rpmmd.PackageSet {
				return rpmmd.PackageSet{
					Include: []string{"nginx"},
				}
			},
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: edgeServices,
		},
		rpmOstree:        true,
		bootISO:          false,
		pipelines:        edgeContainerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "container-tree", "container"},
		exports:          []string{containerPkgsKey},
	}

	edgeRawImgType := imageType{
		name:        "edge-raw-image",
		nameAliases: []string{"rhel-edge-raw-image"},
		filename:    "image.raw.xz",
		mimeType:    "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: edgeRawImageBuildPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: common.StringToPtr("en_US.UTF-8"),
		},
		defaultSize:         10 * common.GibiByte,
		rpmOstree:           true,
		bootable:            true,
		bootISO:             false,
		pipelines:           edgeRawImagePipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"image-tree", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: edgeBasePartitionTables,
	}

	edgeInstallerImgType := imageType{
		name:        "edge-installer",
		nameAliases: []string{"rhel-edge-installer"},
		filename:    "installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			// TODO: non-arch-specific package set handling for installers
			// This image type requires build packages for installers and
			// ostree/edge.  For now we only have x86-64 installer build
			// package sets defined.  When we add installer build package sets
			// for other architectures, this will need to be moved to the
			// architecture and the merging will happen in the PackageSets()
			// method like the other sets.
			buildPkgsKey:     edgeInstallerBuildPackageSet,
			osPkgsKey:        edgeCommitPackageSet,
			installerPkgsKey: edgeInstallerPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale:          common.StringToPtr("en_US.UTF-8"),
			EnabledServices: edgeServices,
		},
		rpmOstree:        true,
		bootISO:          true,
		pipelines:        edgeInstallerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}

	edgeSimplifiedInstallerImgType := imageType{
		name:        "edge-simplified-installer",
		nameAliases: []string{"rhel-edge-simplified-installer"},
		filename:    "simplified-installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			// TODO: non-arch-specific package set handling for installers
			// This image type requires build packages for installers and
			// ostree/edge.  For now we only have x86-64 installer build
			// package sets defined.  When we add installer build package sets
			// for other architectures, this will need to be moved to the
			// architecture and the merging will happen in the PackageSets()
			// method like the other sets.
			buildPkgsKey:     edgeSimplifiedInstallerBuildPackageSet,
			installerPkgsKey: edgeSimplifiedInstallerPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: edgeServices,
		},
		defaultSize:         10 * common.GibiByte,
		rpmOstree:           true,
		bootable:            true,
		bootISO:             true,
		pipelines:           edgeSimplifiedInstallerPipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"image-tree", "image", "archive", "coi-tree", "efiboot-tree", "bootiso-tree", "bootiso"},
		exports:             []string{"bootiso"},
		basePartitionTables: edgeBasePartitionTables,
	}

	qcow2ImgType := imageType{
		name:          "qcow2",
		filename:      "disk.qcow2",
		mimeType:      "application/x-qemu-disk",
		kernelOptions: "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    qcow2CommonPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			DefaultTarget: common.StringToPtr("multi-user.target"),
			RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
				distro.RHSMConfigNoSubscription: {
					DnfPlugins: &osbuild.RHSMStageOptionsDnfPlugins{
						ProductID: &osbuild.RHSMStageOptionsDnfPlugin{
							Enabled: false,
						},
						SubscriptionManager: &osbuild.RHSMStageOptionsDnfPlugin{
							Enabled: false,
						},
					},
				},
			},
		},
		bootable:            true,
		defaultSize:         10 * common.GibiByte,
		pipelines:           qcow2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}

	vmdkImgType := imageType{
		name:     "vmdk",
		filename: "disk.vmdk",
		mimeType: "application/x-vmdk",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    vmdkCommonPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: common.StringToPtr("en_US.UTF-8"),
		},
		kernelOptions:       "ro net.ifnames=0",
		bootable:            true,
		defaultSize:         4 * common.GibiByte,
		pipelines:           vmdkPipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vmdk"},
		exports:             []string{"vmdk"},
		basePartitionTables: defaultBasePartitionTables,
	}

	openstackImgType := imageType{
		name:     "openstack",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    openstackCommonPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: common.StringToPtr("en_US.UTF-8"),
		},
		kernelOptions:       "ro net.ifnames=0",
		bootable:            true,
		defaultSize:         4 * common.GibiByte,
		pipelines:           openstackPipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}

	// default EC2 images config (common for all architectures)
	defaultEc2ImageConfig := &distro.ImageConfig{
		Locale:   common.StringToPtr("en_US.UTF-8"),
		Timezone: common.StringToPtr("UTC"),
		TimeSynchronization: &osbuild.ChronyStageOptions{
			Servers: []osbuild.ChronyConfigServer{
				{
					Hostname: "169.254.169.123",
					Prefer:   common.BoolToPtr(true),
					Iburst:   common.BoolToPtr(true),
					Minpoll:  common.IntToPtr(4),
					Maxpoll:  common.IntToPtr(4),
				},
			},
			// empty string will remove any occurrences of the option from the configuration
			LeapsecTz: common.StringToPtr(""),
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
		DefaultTarget: common.StringToPtr("multi-user.target"),
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
							OnBoot:    common.BoolToPtr(true),
							Type:      osbuild.IfcfgTypeEthernet,
							UserCtl:   common.BoolToPtr(true),
							PeerDNS:   common.BoolToPtr(true),
							IPv6Init:  common.BoolToPtr(false),
						},
					},
				},
			},
		},
		RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
			distro.RHSMConfigNoSubscription: {
				// RHBZ#1932802
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
				// RHBZ#1932802
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					// do not disable the redhat.repo management if the user
					// explicitly request the system to be subscribed
				},
			},
		},
		SystemdLogind: []*osbuild.SystemdLogindStageOptions{
			{
				Filename: "00-getty-fixes.conf",
				Config: osbuild.SystemdLogindConfigDropin{

					Login: osbuild.SystemdLogindConfigLoginSection{
						NAutoVTs: common.IntToPtr(0),
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
				PasswordAuthentication: common.BoolToPtr(false),
			},
		},
	}

	// The RHSM configuration should not be applied since 9.1, but it is instead
	// done by installing the redhat-cloud-client-configuration package.
	// See COMPOSER-1805 for more information.
	rhel91PlusEc2ImageConfigOverride := &distro.ImageConfig{
		RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{},
	}
	if !common.VersionLessThan(rd.osVersion, "9.1") {
		defaultEc2ImageConfig = rhel91PlusEc2ImageConfigOverride.InheritFrom(defaultEc2ImageConfig)
	}

	// default EC2 images config (x86_64)
	defaultEc2ImageConfigX86_64 := &distro.ImageConfig{
		DracutConf: append(defaultEc2ImageConfig.DracutConf,
			&osbuild.DracutConfStageOptions{
				Filename: "ec2.conf",
				Config: osbuild.DracutConfigFile{
					AddDrivers: []string{
						"nvme",
						"xen-blkfront",
					},
				},
			}),
	}
	defaultEc2ImageConfigX86_64 = defaultEc2ImageConfigX86_64.InheritFrom(defaultEc2ImageConfig)

	// default AMI (EC2 BYOS) images config
	defaultAMIImageConfig := &distro.ImageConfig{
		RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
			distro.RHSMConfigNoSubscription: {
				// RHBZ#1932802
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
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
			distro.RHSMConfigWithSubscription: {
				// RHBZ#1932802
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
	defaultAMIImageConfigX86_64 := defaultAMIImageConfig.InheritFrom(defaultEc2ImageConfigX86_64)
	defaultAMIImageConfig = defaultAMIImageConfig.InheritFrom(defaultEc2ImageConfig)

	amiImgTypeX86_64 := imageType{
		name:     "ami",
		filename: "image.raw",
		mimeType: "application/octet-stream",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: ec2BuildPackageSet,
			osPkgsKey:    ec2CommonPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultAMIImageConfigX86_64,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295",
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         10 * common.GibiByte,
		pipelines:           ec2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image"},
		exports:             []string{"image"},
		basePartitionTables: defaultBasePartitionTables,
	}

	amiImgTypeAarch64 := imageType{
		name:     "ami",
		filename: "image.raw",
		mimeType: "application/octet-stream",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: ec2BuildPackageSet,
			osPkgsKey:    ec2CommonPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultAMIImageConfig,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 iommu.strict=0",
		bootable:            true,
		defaultSize:         10 * common.GibiByte,
		pipelines:           ec2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image"},
		exports:             []string{"image"},
		basePartitionTables: defaultBasePartitionTables,
	}

	ec2ImgTypeX86_64 := imageType{
		name:     "ec2",
		filename: "image.raw.xz",
		mimeType: "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: ec2BuildPackageSet,
			osPkgsKey:    rhelEc2PackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultEc2ImageConfigX86_64,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295",
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         10 * common.GibiByte,
		pipelines:           rhelEc2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	ec2ImgTypeAarch64 := imageType{
		name:     "ec2",
		filename: "image.raw.xz",
		mimeType: "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: ec2BuildPackageSet,
			osPkgsKey:    rhelEc2PackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultEc2ImageConfig,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 iommu.strict=0",
		bootable:            true,
		defaultSize:         10 * common.GibiByte,
		pipelines:           rhelEc2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	ec2HaImgTypeX86_64 := imageType{
		name:     "ec2-ha",
		filename: "image.raw.xz",
		mimeType: "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: ec2BuildPackageSet,
			osPkgsKey:    rhelEc2HaPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultEc2ImageConfigX86_64,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295",
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         10 * common.GibiByte,
		pipelines:           rhelEc2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	// default EC2-SAP image config (x86_64)
	defaultEc2SapImageConfigX86_64 := SapImageConfig(rd).InheritFrom(defaultEc2ImageConfigX86_64)

	ec2SapImgTypeX86_64 := imageType{
		name:     "ec2-sap",
		filename: "image.raw.xz",
		mimeType: "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: ec2BuildPackageSet,
			osPkgsKey:    rhelEc2SapPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultEc2SapImageConfigX86_64,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 processor.max_cstate=1 intel_idle.max_cstate=1",
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         10 * common.GibiByte,
		pipelines:           rhelEc2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	defaultGceImageConfig := &distro.ImageConfig{
		Timezone: common.StringToPtr("UTC"),
		TimeSynchronization: &osbuild.ChronyStageOptions{
			Timeservers: []string{"metadata.google.internal"},
		},
		Firewall: &osbuild.FirewallStageOptions{
			DefaultZone: "trusted",
		},
		EnabledServices: []string{
			"sshd",
			"rngd",
			"dnf-automatic.timer",
		},
		DisabledServices: []string{
			"sshd-keygen@",
			"reboot.target",
		},
		DefaultTarget: common.StringToPtr("multi-user.target"),
		Locale:        common.StringToPtr("en_US.UTF-8"),
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
		},
		DNFConfig: []*osbuild.DNFConfigStageOptions{
			{
				Config: &osbuild.DNFConfig{
					Main: &osbuild.DNFConfigMain{
						IPResolve: "4",
					},
				},
			},
		},
		DNFAutomaticConfig: &osbuild.DNFAutomaticConfigStageOptions{
			Config: &osbuild.DNFAutomaticConfig{
				Commands: &osbuild.DNFAutomaticConfigCommands{
					ApplyUpdates: common.BoolToPtr(true),
					UpgradeType:  osbuild.DNFAutomaticUpgradeTypeSecurity,
				},
			},
		},
		YUMRepos: []*osbuild.YumReposStageOptions{
			{
				Filename: "google-cloud.repo",
				Repos: []osbuild.YumRepository{
					{
						Id:      "google-compute-engine",
						Name:    "Google Compute Engine",
						BaseURL: []string{"https://packages.cloud.google.com/yum/repos/google-compute-engine-el9-x86_64-stable"},
						Enabled: common.BoolToPtr(true),
						// TODO: enable GPG check once Google stops using SHA-1 in their keys
						// https://issuetracker.google.com/issues/223626963
						GPGCheck:     common.BoolToPtr(false),
						RepoGPGCheck: common.BoolToPtr(false),
						GPGKey: []string{
							"https://packages.cloud.google.com/yum/doc/yum-key.gpg",
							"https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg",
						},
					},
				},
			},
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
		SshdConfig: &osbuild.SshdConfigStageOptions{
			Config: osbuild.SshdConfigConfig{
				PasswordAuthentication: common.BoolToPtr(false),
				ClientAliveInterval:    common.IntToPtr(420),
				PermitRootLogin:        osbuild.PermitRootLoginValueNo,
			},
		},
		Sysconfig: []*osbuild.SysconfigStageOptions{
			{
				Kernel: &osbuild.SysconfigKernelOptions{
					DefaultKernel: "kernel-core",
					UpdateDefault: true,
				},
			},
		},
		Modprobe: []*osbuild.ModprobeStageOptions{
			{
				Filename: "blacklist-floppy.conf",
				Commands: osbuild.ModprobeConfigCmdList{
					osbuild.NewModprobeConfigCmdBlacklist("floppy"),
				},
			},
		},
		GCPGuestAgentConfig: &osbuild.GcpGuestAgentConfigOptions{
			ConfigScope: osbuild.GcpGuestAgentConfigScopeDistro,
			Config: &osbuild.GcpGuestAgentConfig{
				InstanceSetup: &osbuild.GcpGuestAgentConfigInstanceSetup{
					SetBotoConfig: common.BoolToPtr(false),
				},
			},
		},
	}

	gceImgType := imageType{
		name:     "gce",
		filename: "image.tar.gz",
		mimeType: "application/gzip",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    gcePackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultGceImageConfig,
		kernelOptions:       "net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y console=ttyS0,38400n8d",
		bootable:            true,
		bootType:            distro.UEFIBootType,
		defaultSize:         20 * common.GibiByte,
		pipelines:           gcePipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	defaultGceRhuiImageConfig := &distro.ImageConfig{
		RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
			distro.RHSMConfigNoSubscription: {
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
	defaultGceRhuiImageConfig = defaultGceRhuiImageConfig.InheritFrom(defaultGceImageConfig)

	gceRhuiImgType := imageType{
		name:     "gce-rhui",
		filename: "image.tar.gz",
		mimeType: "application/gzip",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    gceRhuiPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultGceRhuiImageConfig,
		kernelOptions:       "net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y console=ttyS0,38400n8d",
		bootable:            true,
		bootType:            distro.UEFIBootType,
		defaultSize:         20 * common.GibiByte,
		pipelines:           gcePipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	tarImgType := imageType{
		name:     "tar",
		filename: "root.tar.xz",
		mimeType: "application/x-tar",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey: func(t *imageType) rpmmd.PackageSet {
				return rpmmd.PackageSet{
					Include: []string{"policycoreutils", "selinux-policy-targeted"},
					Exclude: []string{"rng-tools"},
				}
			},
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		pipelines:        tarPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "root-tar"},
		exports:          []string{"root-tar"},
	}
	imageInstaller := imageType{
		name:     "image-installer",
		filename: "installer.iso",
		mimeType: "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey:     anacondaBuildPackageSet,
			osPkgsKey:        bareMetalPackageSet,
			installerPkgsKey: anacondaPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		rpmOstree:        false,
		bootISO:          true,
		bootable:         true,
		pipelines:        imageInstallerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "anaconda-tree", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}

	ociImgType := qcow2ImgType
	ociImgType.name = "oci"

	x86_64.addImageTypes(qcow2ImgType, vmdkImgType, openstackImgType, amiImgTypeX86_64, tarImgType, imageInstaller, edgeCommitImgType, edgeInstallerImgType, edgeOCIImgType, edgeRawImgType, edgeSimplifiedInstallerImgType, ociImgType, gceImgType)
	aarch64.addImageTypes(qcow2ImgType, openstackImgType, amiImgTypeAarch64, tarImgType, imageInstaller, edgeCommitImgType, edgeInstallerImgType, edgeOCIImgType, edgeRawImgType, edgeSimplifiedInstallerImgType)
	ppc64le.addImageTypes(qcow2ImgType, tarImgType)
	s390x.addImageTypes(qcow2ImgType, tarImgType)

	if rd.isRHEL() {
		// add azure to RHEL distro only
		x86_64.addImageTypes(azureRhuiImgType)
		x86_64.addImageTypes(azureByosImgType)

		// add ec2 image types to RHEL distro only
		x86_64.addImageTypes(ec2ImgTypeX86_64, ec2HaImgTypeX86_64, ec2SapImgTypeX86_64)
		aarch64.addImageTypes(ec2ImgTypeAarch64)

		// add GCE RHUI image to RHEL only
		x86_64.addImageTypes(gceRhuiImgType)
	} else {
		x86_64.addImageTypes(azureImgType)
	}
	rd.addArches(x86_64, aarch64, ppc64le, s390x)
	return &rd
}
