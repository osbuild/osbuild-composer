package rhel8

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/oscap"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
)

var (
	// rhel8 allow all
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

// RHEL-based OS image configuration defaults
var defaultDistroImageConfig = &distro.ImageConfig{
	Timezone: common.ToPtr("America/New_York"),
	Locale:   common.ToPtr("en_US.UTF-8"),
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

// New creates a new distro object, defining the supported architectures and image types
func New() distro.Distro {
	// default minor: create default minor version (current GA) and rename it
	d := newDistro("rhel", 6)
	d.name = "rhel-8"
	return d

}

func NewRHEL84() distro.Distro {
	return newDistro("rhel", 4)
}

func NewRHEL85() distro.Distro {
	return newDistro("rhel", 5)
}

func NewRHEL86() distro.Distro {
	return newDistro("rhel", 6)
}

func NewRHEL87() distro.Distro {
	return newDistro("rhel", 7)
}

func NewRHEL88() distro.Distro {
	return newDistro("rhel", 8)
}

func NewCentos() distro.Distro {
	return newDistro("centos", 0)
}

func newDistro(name string, minor int) *distribution {
	var rd distribution
	switch name {
	case "rhel":
		rd = distribution{
			name:               fmt.Sprintf("rhel-8%d", minor),
			product:            "Red Hat Enterprise Linux",
			osVersion:          fmt.Sprintf("8.%d", minor),
			releaseVersion:     "8",
			modulePlatformID:   "platform:el8",
			vendor:             "redhat",
			ostreeRefTmpl:      "rhel/8/%s/edge",
			isolabelTmpl:       fmt.Sprintf("RHEL-8-%d-0-BaseOS-%%s", minor),
			runner:             &runner.RHEL{Major: uint64(8), Minor: uint64(minor)},
			defaultImageConfig: defaultDistroImageConfig,
		}
	case "centos":
		rd = distribution{
			name:               "centos-8",
			product:            "CentOS Stream",
			osVersion:          "8-stream",
			releaseVersion:     "8",
			modulePlatformID:   "platform:el8",
			vendor:             "centos",
			ostreeRefTmpl:      "centos/8/%s/edge",
			isolabelTmpl:       "CentOS-Stream-8-%s-dvd",
			runner:             &runner.CentOS{Version: uint64(8)},
			defaultImageConfig: defaultDistroImageConfig,
		}
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

	// GCE BYOS image
	defaultGceByosImageConfig := &distro.ImageConfig{
		Timezone: common.ToPtr("UTC"),
		TimeSynchronization: &osbuild.ChronyStageOptions{
			Servers: []osbuild.ChronyConfigServer{{Hostname: "metadata.google.internal"}},
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
		DefaultTarget: common.ToPtr("multi-user.target"),
		Locale:        common.ToPtr("en_US.UTF-8"),
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
					ApplyUpdates: common.ToPtr(true),
					UpgradeType:  osbuild.DNFAutomaticUpgradeTypeSecurity,
				},
			},
		},
		YUMRepos: []*osbuild.YumReposStageOptions{
			{
				Filename: "google-cloud.repo",
				Repos: []osbuild.YumRepository{
					{
						Id:           "google-compute-engine",
						Name:         "Google Compute Engine",
						BaseURL:      []string{"https://packages.cloud.google.com/yum/repos/google-compute-engine-el8-x86_64-stable"},
						Enabled:      common.ToPtr(true),
						GPGCheck:     common.ToPtr(true),
						RepoGPGCheck: common.ToPtr(false),
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
			distro.RHSMConfigWithSubscription: {
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.ToPtr(true),
					},
					// do not disable the redhat.repo management if the user
					// explicitly request the system to be subscribed
				},
			},
		},
		SshdConfig: &osbuild.SshdConfigStageOptions{
			Config: osbuild.SshdConfigConfig{
				PasswordAuthentication: common.ToPtr(false),
				ClientAliveInterval:    common.ToPtr(420),
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
					SetBotoConfig: common.ToPtr(false),
				},
			},
		},
	}

	if rd.osVersion == "8.4" {
		// NOTE(akoutsou): these are enabled in the package preset, but for
		// some reason do not get enabled on 8.4.
		// the reason is unknown and deeply myserious
		defaultGceByosImageConfig.EnabledServices = append(defaultGceByosImageConfig.EnabledServices,
			"google-oslogin-cache.timer",
			"google-guest-agent.service",
			"google-shutdown-scripts.service",
			"google-startup-scripts.service",
			"google-osconfig-agent.service",
		)
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
		defaultImageConfig:  defaultGceByosImageConfig,
		kernelOptions:       "net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y crashkernel=auto console=ttyS0,38400n8d",
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
						AutoRegistration: common.ToPtr(true),
					},
					Rhsm: &osbuild.SubManConfigRHSMSection{
						ManageRepos: common.ToPtr(false),
					},
				},
			},
			distro.RHSMConfigWithSubscription: {
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
	defaultGceRhuiImageConfig = defaultGceRhuiImageConfig.InheritFrom(defaultGceByosImageConfig)

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
		kernelOptions:       "net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y crashkernel=auto console=ttyS0,38400n8d",
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

	ociImgType := qcow2ImgType(rd)
	ociImgType.name = "oci"

	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "0.10",
			},
		},
		qcow2ImgType(rd),
		ociImgType,
	)

	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
			},
		},
		openstackImgType(),
	)

	rawX86Platform := &platform.X86{
		BIOS: true,
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_RAW,
		},
	}

	x86_64.addImageTypes(
		rawX86Platform,
		amiImgTypeX86_64(rd),
	)

	bareMetalX86Platform := &platform.X86{
		BasePlatform: platform.BasePlatform{
			FirmwarePackages: []string{
				"microcode_ctl", // ??
				"iwl1000-firmware",
				"iwl100-firmware",
				"iwl105-firmware",
				"iwl135-firmware",
				"iwl2000-firmware",
				"iwl2030-firmware",
				"iwl3160-firmware",
				"iwl5000-firmware",
				"iwl5150-firmware",
				"iwl6050-firmware",
			},
		},
		BIOS:       true,
		UEFIVendor: rd.vendor,
	}

	x86_64.addImageTypes(
		bareMetalX86Platform,
		edgeOCIImgType(rd),
		edgeCommitImgType(rd),
		edgeInstallerImgType(rd),
		imageInstaller,
	)

	gceX86Platform := &platform.X86{
		BIOS:       true,
		UEFIVendor: rd.vendor,
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_GCE,
		},
	}

	x86_64.addImageTypes(
		gceX86Platform,
		gceImgType,
	)

	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_VMDK,
			},
		},
		vmdkImgType(),
	)

	x86_64.addImageTypes(
		&platform.X86{},
		tarImgType,
	)

	aarch64.addImageTypes(
		&platform.Aarch64{
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "0.10",
			},
		},
		qcow2ImgType(rd),
	)

	aarch64.addImageTypes(
		&platform.Aarch64{
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
			},
		},
		openstackImgType(),
	)

	aarch64.addImageTypes(
		&platform.Aarch64{},
		tarImgType,
	)

	bareMetalAarch64Platform := &platform.Aarch64{
		BasePlatform: platform.BasePlatform{},
		UEFIVendor:   rd.vendor,
	}

	aarch64.addImageTypes(
		bareMetalAarch64Platform,
		edgeOCIImgType(rd),
		edgeCommitImgType(rd),
		edgeInstallerImgType(rd),
		imageInstaller,
	)

	rawAarch64Platform := &platform.Aarch64{
		UEFIVendor: rd.vendor,
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_RAW,
		},
	}

	aarch64.addImageTypes(
		rawAarch64Platform,
		amiImgTypeAarch64(rd),
	)

	ppc64le.addImageTypes(
		&platform.PPC64LE{
			BIOS: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "0.10",
			},
		},
		qcow2ImgType(rd),
	)

	ppc64le.addImageTypes(
		&platform.PPC64LE{},
		tarImgType,
	)

	s390x.addImageTypes(
		&platform.S390X{
			BIOS: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "0.10",
			},
		},
		qcow2ImgType(rd),
	)

	s390x.addImageTypes(
		&platform.S390X{},
		tarImgType,
	)

	azureX64Platform := &platform.X86{
		BIOS:       true,
		UEFIVendor: rd.vendor,
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_VHD,
		},
	}

	rawUEFIx86Platform := &platform.X86{
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_RAW,
		},
		BIOS:       false,
		UEFIVendor: rd.vendor,
	}

	if rd.isRHEL() {
		if !common.VersionLessThan(rd.osVersion, "8.6") {
			// image types only available on 8.6 and later on RHEL
			// These edge image types require FDO which aren't available on older versions
			x86_64.addImageTypes(
				bareMetalX86Platform,
				edgeRawImgType(),
			)

			x86_64.addImageTypes(
				rawUEFIx86Platform,
				edgeSimplifiedInstallerImgType(rd),
			)

			aarch64.addImageTypes(
				rawAarch64Platform,
				edgeRawImgType(),
				edgeSimplifiedInstallerImgType(rd),
			)
		}

		// add azure to RHEL distro only
		x86_64.addImageTypes(azureX64Platform, azureRhuiImgType, azureByosImgType, azureSapImgType(rd))

		// add ec2 image types to RHEL distro only
		x86_64.addImageTypes(rawX86Platform, ec2ImgTypeX86_64(rd), ec2HaImgTypeX86_64(rd))
		aarch64.addImageTypes(rawAarch64Platform, ec2ImgTypeAarch64(rd))

		if rd.osVersion != "8.5" {
			// NOTE: RHEL 8.5 is going away and these image types require some
			// work to get working, so we just disable them here until the
			// whole distro gets deleted
			x86_64.addImageTypes(rawX86Platform, ec2SapImgTypeX86_64(rd))
		}

		// add GCE RHUI image to RHEL only
		x86_64.addImageTypes(gceX86Platform, gceRhuiImgType)

		// add s390x to RHEL distro only
		rd.addArches(s390x)
	} else {
		x86_64.addImageTypes(
			bareMetalX86Platform,
			edgeRawImgType(),
		)

		x86_64.addImageTypes(
			rawUEFIx86Platform,
			edgeSimplifiedInstallerImgType(rd),
		)

		x86_64.addImageTypes(azureX64Platform, azureImgType)

		aarch64.addImageTypes(
			rawAarch64Platform,
			edgeRawImgType(),
			edgeSimplifiedInstallerImgType(rd),
		)
	}
	rd.addArches(x86_64, aarch64, ppc64le)
	return &rd
}
