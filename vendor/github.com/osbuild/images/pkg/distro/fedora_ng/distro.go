package fedora_ng

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/oscap"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/runner"
)

const (
	// package set names

	// main/common os image package set name
	osPkgsKey = "os"

	// blueprint package set name
	blueprintPkgsKey = "blueprint"

	//Kernel options for ami, qcow2, openstack, vhd and vmdk types
	defaultKernelOptions = "ro no_timer_check console=ttyS0,115200n8 biosdevname=0 net.ifnames=0"
)

var (
	oscapProfileAllowList = []oscap.Profile{
		oscap.Ospp,
		oscap.PciDss,
		oscap.Standard,
	}

	diskRawImgType = imageType{
		name:     "disk-raw",
		filename: "raw.img",
		mimeType: "application/disk",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: diskRawPackageSet,
		},
		rpmOstree:           false,
		kernelOptions:       defaultKernelOptions,
		bootable:            true,
		defaultSize:         2 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image"},
		exports:             []string{"image"},
		basePartitionTables: defaultBasePartitionTables,
	}

	isoLiveImgType = imageType{
		name:        "iso-live",
		nameAliases: []string{},
		filename:    "live.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: isoLivePackageSet,
		},
		bootable:         true,
		bootISO:          true,
		rpmOstree:        false,
		image:            liveImage,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "rootfs-image", "efiboot-tree", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}
)

type distribution struct {
	name               string
	product            string
	osVersion          string
	releaseVersion     string
	modulePlatformID   string
	ostreeRefTmpl      string
	isolabelTmpl       string
	runner             runner.Runner
	arches             map[string]distro.Arch
	defaultImageConfig *distro.ImageConfig
}

// Fedora based OS image configuration defaults
var defaultDistroImageConfig = &distro.ImageConfig{
	Timezone: common.ToPtr("UTC"),
	Locale:   common.ToPtr("en_US"),
}

func getDistro(version int) distribution {
	return distribution{
		name:               fmt.Sprintf("fedora-core-%d", version),
		product:            "Fedora",
		osVersion:          strconv.Itoa(version),
		releaseVersion:     strconv.Itoa(version),
		modulePlatformID:   fmt.Sprintf("platform:f%d", version),
		ostreeRefTmpl:      fmt.Sprintf("fedora/%d/%%s/iot", version),
		isolabelTmpl:       fmt.Sprintf("Fedora-%d-BaseOS-%%s", version),
		runner:             &runner.Fedora{Version: uint64(version)},
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

func (d *distribution) getDefaultImageConfig() *distro.ImageConfig {
	return d.defaultImageConfig
}

type architecture struct {
	distro           *distribution
	name             string
	imageTypes       map[string]distro.ImageType
	imageTypeAliases map[string]string
}

func (a *architecture) Name() string {
	return a.name
}

func (a *architecture) ListImageTypes() []string {
	itNames := make([]string, 0, len(a.imageTypes))
	for name := range a.imageTypes {
		itNames = append(itNames, name)
	}
	sort.Strings(itNames)
	return itNames
}

func (a *architecture) GetImageType(name string) (distro.ImageType, error) {
	t, exists := a.imageTypes[name]
	if !exists {
		aliasForName, exists := a.imageTypeAliases[name]
		if !exists {
			return nil, errors.New("invalid image type: " + name)
		}
		t, exists = a.imageTypes[aliasForName]
		if !exists {
			panic(fmt.Sprintf("image type '%s' is an alias to a non-existing image type '%s'", name, aliasForName))
		}
	}
	return t, nil
}

func (a *architecture) addImageTypes(platform platform.Platform, imageTypes ...imageType) {
	if a.imageTypes == nil {
		a.imageTypes = map[string]distro.ImageType{}
	}
	for idx := range imageTypes {
		it := imageTypes[idx]
		it.arch = a
		it.platform = platform
		a.imageTypes[it.name] = &it
		for _, alias := range it.nameAliases {
			if a.imageTypeAliases == nil {
				a.imageTypeAliases = map[string]string{}
			}
			if existingAliasFor, exists := a.imageTypeAliases[alias]; exists {
				panic(fmt.Sprintf("image type alias '%s' for '%s' is already defined for another image type '%s'", alias, it.name, existingAliasFor))
			}
			a.imageTypeAliases[alias] = it.name
		}
	}
}

func (a *architecture) Distro() distro.Distro {
	return a.distro
}

// New creates a new distro object, defining the supported architectures and image types
func NewF37() distro.Distro {
	return newDistro(37)
}
func NewF38() distro.Distro {
	return newDistro(38)
}
func NewF39() distro.Distro {
	return newDistro(39)
}

func newDistro(version int) distro.Distro {
	rd := getDistro(version)

	// Architecture definitions
	x86_64 := architecture{
		name:   platform.ARCH_X86_64.String(),
		distro: &rd,
	}

	x86_64.addImageTypes(
		&platform.X86{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
		},
		diskRawImgType,
	)

	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: "fedora",
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
					"iwl6000-firmware",
					"iwl6050-firmware",
				},
			},
		},
		isoLiveImgType,
	)

	aarch64 := architecture{
		name:   platform.ARCH_AARCH64.String(),
		distro: &rd,
	}

	aarch64.addImageTypes(
		&platform.Aarch64{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
		},
		diskRawImgType,
	)

	aarch64.addImageTypes(
		&platform.Aarch64{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				FirmwarePackages: []string{
					"uboot-images-armv8", // ??
					"bcm283x-firmware",
					"arm-image-installer", // ??
				},
			},
		},
		isoLiveImgType,
	)

	rd.addArches(x86_64, aarch64)

	return &rd
}
