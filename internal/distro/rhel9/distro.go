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
	"github.com/osbuild/osbuild-composer/internal/platform"
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

	rawX86Platform := &platform.X86{
		BIOS: true,
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_RAW,
		},
	}
	x86_64.addImageTypes(
		rawX86Platform,
		mkAMIImgTypeX86_64(rd.osVersion, rd.isRHEL()),
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
			},
		},
		openstackImgType,
	)

	aarch64.addImageTypes(
		&platform.Aarch64{},
		tarImgType,
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
	)
	aarch64.addImageTypes(
		&platform.Aarch64{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
			UEFIVendor: rd.vendor,
		},
		edgeRawImgType,
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
			BIOS: true,
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

		// add ec2 image types to RHEL distro only
		x86_64.addImageTypes(rawX86Platform, mkEc2ImgTypeX86_64(rd.osVersion, rd.isRHEL()), mkEc2HaImgTypeX86_64(rd.osVersion, rd.isRHEL()), mkEC2SapImgTypeX86_64(rd.osVersion, rd.isRHEL()))

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
		x86_64.addImageTypes(gceX86Platform, gceRhuiImgType)
	} else {
		x86_64.addImageTypes(azureX64Platform, azureImgType)
	}
	rd.addArches(x86_64, aarch64, ppc64le, s390x)
	return &rd
}
