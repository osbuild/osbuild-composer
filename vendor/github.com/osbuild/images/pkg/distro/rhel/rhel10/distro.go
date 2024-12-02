package rhel10

import (
	"fmt"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
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

func distroISOLabelFunc(t *rhel.ImageType) string {
	const RHEL_ISO_LABEL = "RHEL-%s-%s-0-BaseOS-%s"
	const CS_ISO_LABEL = "CentOS-Stream-%s-BaseOS-%s"

	if t.IsRHEL() {
		osVer := strings.Split(t.Arch().Distro().OsVersion(), ".")
		return fmt.Sprintf(RHEL_ISO_LABEL, osVer[0], osVer[1], t.Arch().Name())
	} else {
		return fmt.Sprintf(CS_ISO_LABEL, t.Arch().Distro().Releasever(), t.Arch().Name())
	}
}

func defaultDistroImageConfig(d *rhel.Distribution) *distro.ImageConfig {
	return &distro.ImageConfig{
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
		DefaultOSCAPDatastream: common.ToPtr(oscap.DefaultRHEL10Datastream(d.IsRHEL())),
	}
}

func newDistro(name string, major, minor int) *rhel.Distribution {
	rd, err := rhel.NewDistribution(name, major, minor)
	if err != nil {
		panic(err)
	}
	rd.CheckOptions = checkOptions
	rd.DefaultImageConfig = defaultDistroImageConfig

	// Architecture definitions
	x86_64 := rhel.NewArchitecture(rd, arch.ARCH_X86_64)
	aarch64 := rhel.NewArchitecture(rd, arch.ARCH_AARCH64)
	ppc64le := rhel.NewArchitecture(rd, arch.ARCH_PPC64LE)
	s390x := rhel.NewArchitecture(rd, arch.ARCH_S390X)

	x86_64.AddImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.Vendor(),
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		mkQcow2ImgType(rd),
		mkOCIImgType(rd),
	)

	x86_64.AddImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.Vendor(),
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_VMDK,
			},
		},
		mkVMDKImgType(),
	)

	x86_64.AddImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.Vendor(),
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_OVA,
			},
		},
		mkOVAImgType(),
	)

	x86_64.AddImageTypes(
		&platform.X86{},
		mkTarImgType(),
		mkWSLImgType(),
	)

	aarch64.AddImageTypes(
		&platform.Aarch64{},
		mkTarImgType(),
		mkWSLImgType(),
	)

	aarch64.AddImageTypes(
		&platform.Aarch64{
			UEFIVendor: rd.Vendor(),
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		mkQcow2ImgType(rd),
	)

	ppc64le.AddImageTypes(
		&platform.PPC64LE{
			BIOS: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		mkQcow2ImgType(rd),
	)
	ppc64le.AddImageTypes(
		&platform.PPC64LE{},
		mkTarImgType(),
	)

	s390x.AddImageTypes(
		&platform.S390X{
			Zipl: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		mkQcow2ImgType(rd),
	)
	s390x.AddImageTypes(
		&platform.S390X{},
		mkTarImgType(),
	)

	ec2X86Platform := &platform.X86{
		BIOS:       true,
		UEFIVendor: rd.Vendor(),
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_RAW,
		},
	}
	x86_64.AddImageTypes(
		ec2X86Platform,
		mkAMIImgTypeX86_64(),
	)

	ec2Aarch64Platform := &platform.Aarch64{
		UEFIVendor: rd.Vendor(),
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_RAW,
		},
	}
	aarch64.AddImageTypes(
		ec2Aarch64Platform,
		mkAMIImgTypeAarch64(),
	)

	azureX64Platform := &platform.X86{
		BIOS:       true,
		UEFIVendor: rd.Vendor(),
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_VHD,
		},
	}

	azureAarch64Platform := &platform.Aarch64{
		UEFIVendor: rd.Vendor(),
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_VHD,
		},
	}

	x86_64.AddImageTypes(azureX64Platform, mkAzureImgType(rd))
	aarch64.AddImageTypes(azureAarch64Platform, mkAzureImgType(rd))

	gceX86Platform := &platform.X86{
		UEFIVendor: rd.Vendor(),
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_GCE,
		},
	}
	x86_64.AddImageTypes(
		gceX86Platform,
		mkGCEImageType(),
	)

	x86_64.AddImageTypes(
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
			UEFIVendor: rd.Vendor(),
		},
		mkImageInstallerImgType(),
	)

	aarch64.AddImageTypes(
		&platform.Aarch64{
			BasePlatform: platform.BasePlatform{},
			UEFIVendor:   rd.Vendor(),
		},
		mkImageInstallerImgType(),
	)

	if rd.IsRHEL() { // RHEL-only (non-CentOS) image types
		x86_64.AddImageTypes(azureX64Platform, mkAzureInternalImgType(rd))
		aarch64.AddImageTypes(azureAarch64Platform, mkAzureInternalImgType(rd))

		x86_64.AddImageTypes(azureX64Platform, mkAzureSapInternalImgType(rd))

		x86_64.AddImageTypes(ec2X86Platform, mkEc2ImgTypeX86_64(), mkEc2HaImgTypeX86_64(), mkEC2SapImgTypeX86_64(rd.OsVersion()))
		aarch64.AddImageTypes(ec2Aarch64Platform, mkEC2ImgTypeAarch64())
	}

	rd.AddArches(x86_64, aarch64, ppc64le, s390x)
	return rd
}

func ParseID(idStr string) (*distro.ID, error) {
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

func DistroFactory(idStr string) distro.Distro {
	id, err := ParseID(idStr)
	if err != nil {
		return nil
	}

	return newDistro(id.Name, 10, id.MinorVersion)
}
