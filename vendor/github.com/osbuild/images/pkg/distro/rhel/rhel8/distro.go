package rhel8

import (
	"fmt"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/platform"
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

// RHEL-based OS image configuration defaults
func defaultDistroImageConfig(d *rhel.Distribution) *distro.ImageConfig {
	return common.Must(defs.DistroImageConfig(d.Name()))
}

func distroISOLabelFunc(t *rhel.ImageType) string {
	const RHEL_ISO_LABEL = "RHEL-%s-%s-0-BaseOS-%s"
	const CS_ISO_LABEL = "CentOS-Stream-%s-%s-dvd"

	if t.IsRHEL() {
		osVer := strings.Split(t.Arch().Distro().OsVersion(), ".")
		return fmt.Sprintf(RHEL_ISO_LABEL, osVer[0], osVer[1], t.Arch().Name())
	} else {
		return fmt.Sprintf(CS_ISO_LABEL, t.Arch().Distro().Releasever(), t.Arch().Name())
	}
}

func newDistro(name string, minor int) *rhel.Distribution {
	rd, err := rhel.NewDistribution(name, 8, minor)
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
				QCOW2Compat: "0.10",
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
				ImageFormat: platform.FORMAT_QCOW2,
			},
		},
		mkOpenstackImgType(),
	)

	ec2X86Platform := &platform.X86{
		BIOS:       true,
		UEFIVendor: rd.Vendor(),
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_RAW,
		},
	}

	// Keep the RHEL EC2 x86_64 images before 8.9 BIOS-only for backward compatibility.
	// RHEL-internal EC2 images and RHEL AMI images are kept intentionally in sync
	// with regard to not supporting hybrid boot mode before RHEL version 8.9.
	// The partitioning table for these reflects that and is also intentionally in sync.
	if rd.IsRHEL() && common.VersionLessThan(rd.OsVersion(), "8.9") {
		ec2X86Platform.UEFIVendor = ""
	}

	x86_64.AddImageTypes(
		ec2X86Platform,
		mkAmiImgTypeX86_64(),
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
		UEFIVendor: rd.Vendor(),
	}

	x86_64.AddImageTypes(
		bareMetalX86Platform,
		mkEdgeOCIImgType(rd),
		mkEdgeCommitImgType(rd),
		mkEdgeInstallerImgType(rd),
		mkImageInstaller(),
	)

	gceX86Platform := &platform.X86{
		UEFIVendor: rd.Vendor(),
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_GCE,
		},
	}

	x86_64.AddImageTypes(
		gceX86Platform,
		mkGceImgType(rd),
	)

	x86_64.AddImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.Vendor(),
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_VMDK,
			},
		},
		mkVmdkImgType(),
	)

	x86_64.AddImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.Vendor(),
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_OVA,
			},
		},
		mkOvaImgType(),
	)

	x86_64.AddImageTypes(
		&platform.X86{},
		mkTarImgType(),
		mkWslImgType(),
	)

	aarch64.AddImageTypes(
		&platform.Aarch64{
			UEFIVendor: rd.Vendor(),
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "0.10",
			},
		},
		mkQcow2ImgType(rd),
	)

	aarch64.AddImageTypes(
		&platform.Aarch64{
			UEFIVendor: rd.Vendor(),
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
			},
		},
		mkOpenstackImgType(),
	)

	aarch64.AddImageTypes(
		&platform.Aarch64{},
		mkTarImgType(),
		mkWslImgType(),
	)

	bareMetalAarch64Platform := &platform.Aarch64{
		BasePlatform: platform.BasePlatform{},
		UEFIVendor:   rd.Vendor(),
	}

	aarch64.AddImageTypes(
		bareMetalAarch64Platform,
		mkEdgeOCIImgType(rd),
		mkEdgeCommitImgType(rd),
		mkEdgeInstallerImgType(rd),
		mkImageInstaller(),
	)

	rawAarch64Platform := &platform.Aarch64{
		UEFIVendor: rd.Vendor(),
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_RAW,
		},
	}

	aarch64.AddImageTypes(
		rawAarch64Platform,
		mkAmiImgTypeAarch64(),
		mkMinimalRawImgType(),
	)

	ppc64le.AddImageTypes(
		&platform.PPC64LE{
			BIOS: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "0.10",
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
				QCOW2Compat: "0.10",
			},
		},
		mkQcow2ImgType(rd),
	)

	s390x.AddImageTypes(
		&platform.S390X{},
		mkTarImgType(),
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

	rawUEFIx86Platform := &platform.X86{
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_RAW,
		},
		BIOS:       false,
		UEFIVendor: rd.Vendor(),
	}

	x86_64.AddImageTypes(
		rawUEFIx86Platform,
		mkMinimalRawImgType(),
	)

	if rd.IsRHEL() {
		if common.VersionGreaterThanOrEqual(rd.OsVersion(), "8.6") {
			// image types only available on 8.6 and later on RHEL
			// These edge image types require FDO which aren't available on older versions
			x86_64.AddImageTypes(
				bareMetalX86Platform,
				mkEdgeRawImgType(),
			)

			x86_64.AddImageTypes(
				rawUEFIx86Platform,
				mkEdgeSimplifiedInstallerImgType(rd),
			)

			x86_64.AddImageTypes(
				azureX64Platform,
				mkAzureEap7RhuiImgType(),
			)

			aarch64.AddImageTypes(
				rawAarch64Platform,
				mkEdgeRawImgType(),
				mkEdgeSimplifiedInstallerImgType(rd),
			)

			// The Azure image types require hyperv-daemons which isn't available on older versions
			aarch64.AddImageTypes(
				azureAarch64Platform,
				mkAzureRhuiImgType(),
				mkAzureByosImgType(),
			)
		}

		// add azure to RHEL distro only
		x86_64.AddImageTypes(
			azureX64Platform,
			mkAzureRhuiImgType(),
			mkAzureByosImgType(),
			mkAzureSapRhuiImgType(rd),
		)

		// add ec2 image types to RHEL distro only
		x86_64.AddImageTypes(
			ec2X86Platform,
			mkEc2ImgTypeX86_64(rd),
			mkEc2HaImgTypeX86_64(rd),
		)
		aarch64.AddImageTypes(
			rawAarch64Platform,
			mkEc2ImgTypeAarch64(rd),
		)

		if rd.OsVersion() != "8.5" {
			// NOTE: RHEL 8.5 is going away and these image types require some
			// work to get working, so we just disable them here until the
			// whole distro gets deleted
			x86_64.AddImageTypes(
				ec2X86Platform,
				mkEc2SapImgTypeX86_64(rd),
			)
		}

		// add GCE RHUI image to RHEL only
		x86_64.AddImageTypes(
			gceX86Platform,
			mkGceRhuiImgType(rd),
		)

		// add s390x to RHEL distro only
		rd.AddArches(s390x)
	} else {
		x86_64.AddImageTypes(
			bareMetalX86Platform,
			mkEdgeRawImgType(),
		)

		x86_64.AddImageTypes(
			rawUEFIx86Platform,
			mkEdgeSimplifiedInstallerImgType(rd),
		)

		x86_64.AddImageTypes(
			azureX64Platform,
			mkAzureImgType(),
		)

		aarch64.AddImageTypes(
			rawAarch64Platform,
			mkEdgeRawImgType(),
			mkEdgeSimplifiedInstallerImgType(rd),
		)

		aarch64.AddImageTypes(
			azureAarch64Platform,
			mkAzureImgType(),
		)
	}
	rd.AddArches(x86_64, aarch64, ppc64le)
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

	// Backward compatibility layer for "rhel-84" or "rhel-810"
	if id.Name == "rhel" && id.MinorVersion == -1 {
		if id.MajorVersion/10 == 8 {
			// handle single digit minor version
			id.MinorVersion = id.MajorVersion % 10
			id.MajorVersion = 8
		} else if id.MajorVersion/100 == 8 {
			// handle two digit minor version
			id.MinorVersion = id.MajorVersion % 100
			id.MajorVersion = 8
		}
	}

	if id.MajorVersion != 8 {
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

	return newDistro(id.Name, id.MinorVersion)
}
