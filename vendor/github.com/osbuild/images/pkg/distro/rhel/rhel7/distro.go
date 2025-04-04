package rhel7

import (
	"fmt"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/platform"
)

// RHEL-based OS image configuration defaults
func defaultDistroImageConfig(d *rhel.Distribution) *distro.ImageConfig {
	return &distro.ImageConfig{
		Timezone: common.ToPtr("America/New_York"),
		Locale:   common.ToPtr("en_US.UTF-8"),
		GPGKeyFiles: []string{
			"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
		},
		UpdateDefaultKernel: common.ToPtr(true),
		DefaultKernel:       common.ToPtr("kernel"),
		Sysconfig: &distro.Sysconfig{
			Networking: true,
			NoZeroConf: true,
		},
		KernelOptionsBootloader: common.ToPtr(true),
		NoBLS:                   common.ToPtr(true), // RHEL 7 grub does not support BLS
		InstallWeakDeps:         common.ToPtr(true),
	}
}

func newDistro(name string, minor int) *rhel.Distribution {
	rd, err := rhel.NewDistribution(name, 7, minor)
	if err != nil {
		panic(err)
	}

	rd.CheckOptions = checkOptions
	rd.DefaultImageConfig = defaultDistroImageConfig
	rd.DistCodename = "Maipo"

	// Architecture definitions
	x86_64 := rhel.NewArchitecture(rd, arch.ARCH_X86_64)

	x86_64.AddImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.Vendor(),
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "0.10",
			},
		},
		mkQcow2ImgType(),
	)

	x86_64.AddImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: rd.Vendor(),
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_VHD,
			},
		},
		mkAzureRhuiImgType(),
	)

	x86_64.AddImageTypes(
		&platform.X86{
			BIOS: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
		},
		mkEc2ImgTypeX86_64(),
	)

	rd.AddArches(
		x86_64,
	)

	return rd
}

func ParseID(idStr string) (*distro.ID, error) {
	id, err := distro.ParseID(idStr)
	if err != nil {
		return nil, err
	}
	if id.Name != "rhel" {
		return nil, fmt.Errorf("invalid distro name: %s", id.Name)
	}

	if id.MajorVersion != 7 {
		return nil, fmt.Errorf("invalid distro major version: %d", id.MajorVersion)
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
