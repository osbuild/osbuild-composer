package rhel8

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/runner"
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
	d := newDistro("rhel", 10)
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

func NewRHEL89() distro.Distro {
	return newDistro("rhel", 9)
}

func NewRHEL810() distro.Distro {
	return newDistro("rhel", 10)
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
		name:   arch.ARCH_X86_64.String(),
		distro: &rd,
	}

	aarch64 := architecture{
		name:   arch.ARCH_AARCH64.String(),
		distro: &rd,
	}

	ppc64le := architecture{
		distro: &rd,
		name:   arch.ARCH_PPC64LE.String(),
	}
	s390x := architecture{
		distro: &rd,
		name:   arch.ARCH_S390X.String(),
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

	ec2X86Platform := &platform.X86{
		BIOS:       true,
		UEFIVendor: rd.vendor,
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_RAW,
		},
	}
	x86_64.addImageTypes(
		ec2X86Platform,
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
		imageInstaller(),
	)

	gceX86Platform := &platform.X86{
		UEFIVendor: rd.vendor,
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_GCE,
		},
	}

	x86_64.addImageTypes(
		gceX86Platform,
		gceImgType(rd),
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
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_OVA,
			},
		},
		ovaImgType(),
	)

	x86_64.addImageTypes(
		&platform.X86{},
		tarImgType(),
		wslImgType(),
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
		tarImgType(),
		wslImgType(),
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
		imageInstaller(),
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
		minimalRawImgType(rd),
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
		tarImgType(),
	)

	s390x.addImageTypes(
		&platform.S390X{
			Zipl: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "0.10",
			},
		},
		qcow2ImgType(rd),
	)

	s390x.addImageTypes(
		&platform.S390X{},
		tarImgType(),
	)

	azureX64Platform := &platform.X86{
		BIOS:       true,
		UEFIVendor: rd.vendor,
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_VHD,
		},
	}

	azureAarch64Platform := &platform.Aarch64{
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

	x86_64.addImageTypes(
		rawUEFIx86Platform,
		minimalRawImgType(rd),
	)

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

			azureEap := azureEap7RhuiImgType()
			x86_64.addImageTypes(azureX64Platform, azureEap)

			aarch64.addImageTypes(
				rawAarch64Platform,
				edgeRawImgType(),
				edgeSimplifiedInstallerImgType(rd),
			)

			// The Azure image types require hyperv-daemons which isn't available on older versions
			aarch64.addImageTypes(azureAarch64Platform, azureRhuiImgType(), azureByosImgType())
		}

		// add azure to RHEL distro only
		x86_64.addImageTypes(azureX64Platform, azureRhuiImgType(), azureByosImgType(), azureSapRhuiImgType(rd))

		// keep the RHEL EC2 x86_64 images before 8.9 BIOS-only for backward compatibility
		if common.VersionLessThan(rd.osVersion, "8.9") {
			ec2X86Platform = &platform.X86{
				BIOS: true,
				BasePlatform: platform.BasePlatform{
					ImageFormat: platform.FORMAT_RAW,
				},
			}
		}

		// add ec2 image types to RHEL distro only
		x86_64.addImageTypes(ec2X86Platform, ec2ImgTypeX86_64(rd), ec2HaImgTypeX86_64(rd))
		aarch64.addImageTypes(rawAarch64Platform, ec2ImgTypeAarch64(rd))

		if rd.osVersion != "8.5" {
			// NOTE: RHEL 8.5 is going away and these image types require some
			// work to get working, so we just disable them here until the
			// whole distro gets deleted
			x86_64.addImageTypes(ec2X86Platform, ec2SapImgTypeX86_64(rd))
		}

		// add GCE RHUI image to RHEL only
		x86_64.addImageTypes(gceX86Platform, gceRhuiImgType(rd))

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

		x86_64.addImageTypes(azureX64Platform, azureImgType())

		aarch64.addImageTypes(
			rawAarch64Platform,
			edgeRawImgType(),
			edgeSimplifiedInstallerImgType(rd),
		)

		aarch64.addImageTypes(azureAarch64Platform, azureImgType())
	}
	rd.addArches(x86_64, aarch64, ppc64le)
	return &rd
}
