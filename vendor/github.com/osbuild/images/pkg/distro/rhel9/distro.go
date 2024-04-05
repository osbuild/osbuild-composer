package rhel9

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
	// rhel9 & cs9 share the same list
	// of allowed profiles so a single
	// allow list can be used
	oscapProfileAllowList = []oscap.Profile{
		oscap.AnssiBp28Enhanced,
		oscap.AnssiBp28High,
		oscap.AnssiBp28Intermediary,
		oscap.AnssiBp28Minimal,
		oscap.CcnAdvanced,
		oscap.CcnBasic,
		oscap.CcnIntermediate,
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

func (d *distribution) OsVersion() string {
	return d.osVersion
}

func (d *distribution) Product() string {
	return d.product
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

func newDistro(name string, major, minor int) *distribution {
	var rd distribution
	switch fmt.Sprintf("%s-%d", name, major) {
	case "rhel-9":
		rd = distribution{
			name:               fmt.Sprintf("rhel-9.%d", minor),
			product:            "Red Hat Enterprise Linux",
			osVersion:          fmt.Sprintf("9.%d", minor),
			releaseVersion:     "9",
			modulePlatformID:   "platform:el9",
			vendor:             "redhat",
			ostreeRefTmpl:      "rhel/9/%s/edge",
			runner:             &runner.RHEL{Major: uint64(9), Minor: uint64(minor)},
			defaultImageConfig: defaultDistroImageConfig,
		}
	case "rhel-10":
		rd = distribution{
			name:               fmt.Sprintf("rhel-10.%d", minor),
			product:            "Red Hat Enterprise Linux",
			osVersion:          fmt.Sprintf("10.%d", minor),
			releaseVersion:     "10",
			modulePlatformID:   "platform:el10",
			vendor:             "redhat",
			ostreeRefTmpl:      "rhel/10/%s/edge",
			runner:             &runner.RHEL{Major: uint64(10), Minor: uint64(minor)},
			defaultImageConfig: defaultDistroImageConfig,
		}
	case "centos-9":
		rd = distribution{
			name:               "centos-9",
			product:            "CentOS Stream",
			osVersion:          "9-stream",
			releaseVersion:     "9",
			modulePlatformID:   "platform:el9",
			vendor:             "centos",
			ostreeRefTmpl:      "centos/9/%s/edge",
			runner:             &runner.CentOS{Version: uint64(9)},
			defaultImageConfig: defaultDistroImageConfig,
		}
	case "centos-10":
		rd = distribution{
			name:               "centos-10",
			product:            "CentOS Stream",
			osVersion:          "10-stream",
			releaseVersion:     "10",
			modulePlatformID:   "platform:el10",
			vendor:             "centos",
			ostreeRefTmpl:      "centos/10/%s/edge",
			runner:             &runner.CentOS{Version: uint64(10)},
			defaultImageConfig: defaultDistroImageConfig,
		}
	default:
		panic(fmt.Sprintf("unknown distro name: %s and major: %d", name, major))
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
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		qcow2ImgType,
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

	ec2X86Platform := &platform.X86{
		BIOS:       true,
		UEFIVendor: rd.vendor,
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_RAW,
		},
	}
	x86_64.addImageTypes(
		ec2X86Platform,
		mkAMIImgTypeX86_64(),
	)

	aarch64.addImageTypes(
		&platform.Aarch64{
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
		},
		mkAMIImgTypeAarch64(),
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

	if rd.isRHEL() { // RHEL-only (non-CentOS) image types
		x86_64.addImageTypes(azureX64Platform, azureByosImgType(rd))
		aarch64.addImageTypes(azureAarch64Platform, azureByosImgType(rd))
	} else {
		x86_64.addImageTypes(azureX64Platform, azureImgType)
		aarch64.addImageTypes(azureAarch64Platform, azureImgType)
	}

	// NOTE: This condition is a temporary separation of EL9 and EL10 while we
	// add support for all image types on EL10.  Currently only a small subset
	// is supported on EL10 because of package availability.  This big
	// conditional separation should be removed when most image types become
	// available in EL10.
	if major == 9 {
		gceX86Platform := &platform.X86{
			UEFIVendor: rd.vendor,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_GCE,
			},
		}
		x86_64.addImageTypes(
			gceX86Platform,
			mkGCEImageType(),
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

		if rd.isRHEL() { // RHEL-only (non-CentOS) image types
			x86_64.addImageTypes(azureX64Platform, azureRhuiImgType, azureByosImgType(rd))
			aarch64.addImageTypes(azureAarch64Platform, azureRhuiImgType, azureByosImgType(rd))

			x86_64.addImageTypes(azureX64Platform, azureSapRhuiImgType(rd))

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
			x86_64.addImageTypes(gceX86Platform, mkGCERHUIImageType())
		}
	}

	rd.addArches(x86_64, aarch64, ppc64le, s390x)
	return &rd
}

func ParseID(idStr string) (*distro.ID, error) {
	id, err := distro.ParseID(idStr)
	if err != nil {
		return nil, err
	}

	if id.Name != "rhel" && id.Name != "centos" {
		return nil, fmt.Errorf("invalid distro name: %s", id.Name)
	}

	// Backward compatibility layer for "rhel-93" or "rhel-910"
	if id.Name == "rhel" && id.MinorVersion == -1 {
		if id.MajorVersion/10 == 9 {
			// handle single digit minor version
			id.MinorVersion = id.MajorVersion % 10
			id.MajorVersion = 9
		} else if id.MajorVersion/100 == 9 {
			// handle two digit minor version
			id.MinorVersion = id.MajorVersion % 100
			id.MajorVersion = 9
		}
	}

	if id.MajorVersion != 9 {
		return nil, fmt.Errorf("invalid distro major version: %d", id.MajorVersion)
	}

	// CentOS does not use minor version
	if id.Name == "centos" && id.MinorVersion != -1 {
		return nil, fmt.Errorf("centos does not use minor version, but got: %d", id.MinorVersion)
	}

	// RHEL uses minor version
	if id.Name == "rhel" && id.MinorVersion == -1 {
		return nil, fmt.Errorf("rhel requires minor version, but got: %d", id.MinorVersion)
	}

	return id, nil
}

func DistroFactory(idStr string) distro.Distro {
	id, err := ParseID(idStr)
	if err != nil {
		return nil
	}

	return newDistro(id.Name, 9, id.MinorVersion)
}

func ParseIDEl10(idStr string) (*distro.ID, error) {
	id, err := distro.ParseID(idStr)
	if err != nil {
		return nil, err
	}

	if id.Name != "rhel" && id.Name != "centos" {
		return nil, fmt.Errorf("invalid distro name: %s", id.Name)
	}

	if id.MajorVersion != 10 {
		return nil, fmt.Errorf("invalid distro major version: %d", id.MajorVersion)
	}

	// CentOS does not use minor version
	if id.Name == "centos" && id.MinorVersion != -1 {
		return nil, fmt.Errorf("centos does not use minor version, but got: %d", id.MinorVersion)
	}

	// RHEL uses minor version
	if id.Name == "rhel" && id.MinorVersion == -1 {
		return nil, fmt.Errorf("rhel requires minor version, but got: %d", id.MinorVersion)
	}

	return id, nil
}

func DistroFactoryEl10(idStr string) distro.Distro {
	id, err := ParseIDEl10(idStr)
	if err != nil {
		return nil
	}

	return newDistro(id.Name, 10, id.MinorVersion)
}
