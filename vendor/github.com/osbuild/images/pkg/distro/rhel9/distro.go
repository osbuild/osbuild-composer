package rhel9

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/oscap"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/runner"
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
	Timezone: common.ToPtr("America/New_York"),
	Locale:   common.ToPtr("C.UTF-8"),
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

func New() distro.Distro {
	// default minor: create default minor version (current GA) and rename it
	d := newDistro("rhel", 4)
	d.name = "rhel-9"
	return d
}

func NewCentOS9() distro.Distro {
	return newDistro("centos", 0)
}

func NewRHEL90() distro.Distro {
	return newDistro("rhel", 0)
}

func NewRHEL91() distro.Distro {
	return newDistro("rhel", 1)
}

func NewRHEL92() distro.Distro {
	return newDistro("rhel", 2)
}

func NewRHEL93() distro.Distro {
	return newDistro("rhel", 3)
}

func NewRHEL94() distro.Distro {
	return newDistro("rhel", 4)
}

func newDistro(name string, minor int) *distribution {
	var rd distribution
	switch name {
	case "rhel":
		rd = distribution{
			name:               fmt.Sprintf("rhel-9%d", minor),
			product:            "Red Hat Enterprise Linux",
			osVersion:          fmt.Sprintf("9.%d", minor),
			releaseVersion:     "9",
			modulePlatformID:   "platform:el9",
			vendor:             "redhat",
			ostreeRefTmpl:      "rhel/9/%s/edge",
			isolabelTmpl:       fmt.Sprintf("RHEL-9-%d-0-BaseOS-%%s", minor),
			runner:             &runner.RHEL{Major: uint64(9), Minor: uint64(minor)},
			defaultImageConfig: defaultDistroImageConfig,
		}
	case "centos":
		rd = distribution{
			name:               "centos-9",
			product:            "CentOS Stream",
			osVersion:          "9-stream",
			releaseVersion:     "9",
			modulePlatformID:   "platform:el9",
			vendor:             "centos",
			ostreeRefTmpl:      "centos/9/%s/edge",
			isolabelTmpl:       "CentOS-Stream-9-BaseOS-%s",
			runner:             &runner.CentOS{Version: uint64(9)},
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

	qcow2ImgType := mkQcow2ImgType(rd)
	ociImgType := qcow2ImgType
	ociImgType.name = "oci"

	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		qcow2ImgType,
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
		openstackImgType,
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

	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_VMDK,
			},
		},
		vmdkImgType,
	)

	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_OVA,
			},
		},
		ovaImgType,
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
		mkAMIImgTypeX86_64(rd.osVersion, rd.isRHEL()),
	)

	gceX86Platform := &platform.X86{
		UEFIVendor: rd.vendor,
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_GCE,
		},
	}
	x86_64.addImageTypes(
		gceX86Platform,
		mkGCEImageType(rd.isRHEL()),
	)

	x86_64.addImageTypes(
		&platform.X86{
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
		},
		edgeOCIImgType,
		edgeCommitImgType,
		edgeInstallerImgType,
		edgeRawImgType,
		imageInstaller,
		edgeAMIImgType,
	)

	x86_64.addImageTypes(
		&platform.X86{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_VMDK,
			},
			BIOS:       true,
			UEFIVendor: rd.vendor,
		},
		edgeVsphereImgType,
	)

	x86_64.addImageTypes(
		&platform.X86{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
			BIOS:       false,
			UEFIVendor: rd.vendor,
		},
		edgeSimplifiedInstallerImgType,
		minimalrawImgType,
	)

	x86_64.addImageTypes(
		&platform.X86{},
		tarImgType,
		wslImgType,
	)

	aarch64.addImageTypes(
		&platform.Aarch64{
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
			},
		},
		openstackImgType,
	)

	aarch64.addImageTypes(
		&platform.Aarch64{},
		tarImgType,
		wslImgType,
	)

	aarch64.addImageTypes(
		&platform.Aarch64{
			BasePlatform: platform.BasePlatform{},
			UEFIVendor:   rd.vendor,
		},
		edgeCommitImgType,
		edgeOCIImgType,
		edgeInstallerImgType,
		edgeSimplifiedInstallerImgType,
		imageInstaller,
		edgeAMIImgType,
	)

	aarch64.addImageTypes(
		&platform.Aarch64{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_VMDK,
			},
			UEFIVendor: rd.vendor,
		},
		edgeVsphereImgType,
	)

	aarch64.addImageTypes(
		&platform.Aarch64{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
			UEFIVendor: rd.vendor,
		},
		edgeRawImgType,
		minimalrawImgType,
	)

	aarch64.addImageTypes(
		&platform.Aarch64{
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		qcow2ImgType,
	)
	aarch64.addImageTypes(
		&platform.Aarch64{
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
		},
		mkAMIImgTypeAarch64(rd.osVersion, rd.isRHEL()),
	)

	ppc64le.addImageTypes(
		&platform.PPC64LE{
			BIOS: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		qcow2ImgType,
	)
	ppc64le.addImageTypes(
		&platform.PPC64LE{},
		tarImgType,
	)

	s390x.addImageTypes(
		&platform.S390X{
			Zipl: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		qcow2ImgType,
	)
	s390x.addImageTypes(
		&platform.S390X{},
		tarImgType,
	)

	if rd.isRHEL() {
		// add azure to RHEL distro only
		x86_64.addImageTypes(azureX64Platform, azureRhuiImgType, azureByosImgType)
		aarch64.addImageTypes(azureAarch64Platform, azureRhuiImgType, azureByosImgType)

		// keep the RHEL EC2 x86_64 images before 9.3 BIOS-only for backward compatibility
		if common.VersionLessThan(rd.osVersion, "9.3") {
			ec2X86Platform = &platform.X86{
				BIOS: true,
				BasePlatform: platform.BasePlatform{
					ImageFormat: platform.FORMAT_RAW,
				},
			}
		}

		// add ec2 image types to RHEL distro only
		x86_64.addImageTypes(ec2X86Platform, mkEc2ImgTypeX86_64(rd.osVersion, rd.isRHEL()), mkEc2HaImgTypeX86_64(rd.osVersion, rd.isRHEL()), mkEC2SapImgTypeX86_64(rd.osVersion, rd.isRHEL()))

		aarch64.addImageTypes(
			&platform.Aarch64{
				UEFIVendor: rd.vendor,
				BasePlatform: platform.BasePlatform{
					ImageFormat: platform.FORMAT_RAW,
				},
			},
			mkEC2ImgTypeAarch64(rd.osVersion, rd.isRHEL()),
		)

		// add GCE RHUI image to RHEL only
		x86_64.addImageTypes(gceX86Platform, mkGCERHUIImageType(rd.isRHEL()))
	} else {
		x86_64.addImageTypes(azureX64Platform, azureImgType)
		aarch64.addImageTypes(azureAarch64Platform, azureImgType)
	}
	rd.addArches(x86_64, aarch64, ppc64le, s390x)
	return &rd
}
