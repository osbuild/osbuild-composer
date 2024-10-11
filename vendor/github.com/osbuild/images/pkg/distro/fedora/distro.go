package fedora

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

const (
	// package set names

	// main/common os image package set name
	osPkgsKey = "os"

	// container package set name
	containerPkgsKey = "container"

	// installer package set name
	installerPkgsKey = "installer"

	// blueprint package set name
	blueprintPkgsKey = "blueprint"

	//Default kernel command line
	defaultKernelOptions = "ro"

	// Added kernel command line options for ami, qcow2, openstack, vhd and vmdk types
	cloudKernelOptions = "ro no_timer_check console=ttyS0,115200n8 biosdevname=0 net.ifnames=0"

	// Added kernel command line options for iot-raw-image and iot-qcow2-image types
	ostreeDeploymentKernelOptions = "modprobe.blacklist=vc4 rw coreos.no_persist_ip"
)

var (
	oscapProfileAllowList = []oscap.Profile{
		oscap.Ospp,
		oscap.PciDss,
		oscap.Standard,
	}

	// Services
	iotServices = []string{
		"NetworkManager.service",
		"firewalld.service",
		"sshd.service",
		"zezere_ignition.timer",
		"zezere_ignition_banner.service",
		"greenboot-grub2-set-counter",
		"greenboot-grub2-set-success",
		"greenboot-healthcheck",
		"greenboot-rpm-ostree-grub2-check-fallback",
		"greenboot-status",
		"greenboot-task-runner",
		"redboot-auto-reboot",
		"redboot-task-runner",
		"parsec",
		"dbus-parsec",
	}

	minimalRawServices = []string{
		"NetworkManager.service",
		"firewalld.service",
		"initial-setup.service",
		"sshd.service",
	}

	// Image Definitions
	imageInstallerImgType = imageType{
		name:        "image-installer",
		nameAliases: []string{"fedora-image-installer"},
		filename:    "installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			osPkgsKey:        minimalrpmPackageSet,
			installerPkgsKey: imageInstallerPackageSet,
		},
		bootable:  true,
		bootISO:   true,
		rpmOstree: false,
		image:     imageInstallerImage,
		// We don't know the variant of the OS pipeline being installed
		isoLabel:         getISOLabelFunc("Unknown"),
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "rootfs-image", "efiboot-tree", "os", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}

	liveInstallerImgType = imageType{
		name:        "live-installer",
		nameAliases: []string{},
		filename:    "live-installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			installerPkgsKey: liveInstallerPackageSet,
		},
		bootable:         true,
		bootISO:          true,
		rpmOstree:        false,
		image:            liveInstallerImage,
		isoLabel:         getISOLabelFunc("Workstation"),
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "rootfs-image", "efiboot-tree", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}

	iotCommitImgType = imageType{
		name:        "iot-commit",
		nameAliases: []string{"fedora-iot-commit"},
		filename:    "commit.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: iotCommitPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: iotServices,
			DracutConf:      []*osbuild.DracutConfStageOptions{osbuild.FIPSDracutConfStageOptions},
		},
		rpmOstree:        true,
		image:            iotCommitImage,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "ostree-commit", "commit-archive"},
		exports:          []string{"commit-archive"},
	}

	iotBootableContainer = imageType{
		name:     "iot-bootable-container",
		filename: "iot-bootable-container.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: bootableContainerPackageSet,
		},
		rpmOstree:        true,
		image:            bootableContainerImage,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "ostree-commit", "ostree-encapsulate"},
		exports:          []string{"ostree-encapsulate"},
	}

	iotOCIImgType = imageType{
		name:        "iot-container",
		nameAliases: []string{"fedora-iot-container"},
		filename:    "container.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: iotCommitPackageSet,
			containerPkgsKey: func(t *imageType) rpmmd.PackageSet {
				return rpmmd.PackageSet{}
			},
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: iotServices,
			DracutConf:      []*osbuild.DracutConfStageOptions{osbuild.FIPSDracutConfStageOptions},
		},
		rpmOstree:        true,
		bootISO:          false,
		image:            iotContainerImage,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "ostree-commit", "container-tree", "container"},
		exports:          []string{"container"},
	}

	iotInstallerImgType = imageType{
		name:        "iot-installer",
		nameAliases: []string{"fedora-iot-installer"},
		filename:    "installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			installerPkgsKey: iotInstallerPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale:          common.ToPtr("en_US.UTF-8"),
			EnabledServices: iotServices,
		},
		rpmOstree:        true,
		bootISO:          true,
		image:            iotInstallerImage,
		isoLabel:         getISOLabelFunc("IoT"),
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "rootfs-image", "efiboot-tree", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}

	iotSimplifiedInstallerImgType = imageType{
		name:     "iot-simplified-installer",
		filename: "simplified-installer.iso",
		mimeType: "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			installerPkgsKey: iotSimplifiedInstallerPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: iotServices,
			Keyboard: &osbuild.KeymapStageOptions{
				Keymap: "us",
			},
			Locale:                    common.ToPtr("C.UTF-8"),
			OSTreeConfSysrootReadOnly: common.ToPtr(true),
			LockRootUser:              common.ToPtr(true),
			IgnitionPlatform:          common.ToPtr("metal"),
		},
		defaultSize:         10 * common.GibiByte,
		rpmOstree:           true,
		bootable:            true,
		bootISO:             true,
		image:               iotSimplifiedInstallerImage,
		isoLabel:            getISOLabelFunc("IoT"),
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"ostree-deployment", "image", "xz", "coi-tree", "efiboot-tree", "bootiso-tree", "bootiso"},
		exports:             []string{"bootiso"},
		basePartitionTables: iotSimplifiedInstallerPartitionTables,
		kernelOptions:       ostreeDeploymentKernelOptions,
	}

	iotRawImgType = imageType{
		name:        "iot-raw-image",
		nameAliases: []string{"fedora-iot-raw-image"},
		filename:    "image.raw.xz",
		compression: "xz",
		mimeType:    "application/xz",
		packageSets: map[string]packageSetFunc{},
		defaultImageConfig: &distro.ImageConfig{
			Keyboard: &osbuild.KeymapStageOptions{
				Keymap: "us",
			},
			Locale:                    common.ToPtr("C.UTF-8"),
			OSTreeConfSysrootReadOnly: common.ToPtr(true),
			LockRootUser:              common.ToPtr(true),
			IgnitionPlatform:          common.ToPtr("metal"),
		},
		defaultSize:         4 * common.GibiByte,
		rpmOstree:           true,
		bootable:            true,
		image:               iotImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"ostree-deployment", "image", "xz"},
		exports:             []string{"xz"},
		basePartitionTables: iotBasePartitionTables,
		kernelOptions:       ostreeDeploymentKernelOptions,

		// Passing an empty map into the required partition sizes disables the
		// default partition sizes normally set so our `basePartitionTables` can
		// override them (and make them smaller, in this case).
		requiredPartitionSizes: map[string]uint64{},
	}

	iotQcow2ImgType = imageType{
		name:        "iot-qcow2-image",
		filename:    "image.qcow2",
		mimeType:    "application/x-qemu-disk",
		packageSets: map[string]packageSetFunc{},
		defaultImageConfig: &distro.ImageConfig{
			Keyboard: &osbuild.KeymapStageOptions{
				Keymap: "us",
			},
			Locale:                    common.ToPtr("C.UTF-8"),
			OSTreeConfSysrootReadOnly: common.ToPtr(true),
			LockRootUser:              common.ToPtr(true),
			IgnitionPlatform:          common.ToPtr("qemu"),
		},
		defaultSize:         10 * common.GibiByte,
		rpmOstree:           true,
		bootable:            true,
		image:               iotImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"ostree-deployment", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: iotBasePartitionTables,
		kernelOptions:       ostreeDeploymentKernelOptions,
	}

	qcow2ImgType = imageType{
		name:        "qcow2",
		filename:    "disk.qcow2",
		mimeType:    "application/x-qemu-disk",
		environment: &environment.KVM{},
		packageSets: map[string]packageSetFunc{
			osPkgsKey: qcow2CommonPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			DefaultTarget: common.ToPtr("multi-user.target"),
		},
		kernelOptions:       cloudKernelOptions,
		bootable:            true,
		defaultSize:         5 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}

	vmdkDefaultImageConfig = &distro.ImageConfig{
		Locale: common.ToPtr("en_US.UTF-8"),
		EnabledServices: []string{
			"cloud-init.service",
			"cloud-config.service",
			"cloud-final.service",
			"cloud-init-local.service",
		},
	}

	vmdkImgType = imageType{
		name:     "vmdk",
		filename: "disk.vmdk",
		mimeType: "application/x-vmdk",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: vmdkCommonPackageSet,
		},
		defaultImageConfig:  vmdkDefaultImageConfig,
		kernelOptions:       cloudKernelOptions,
		bootable:            true,
		defaultSize:         2 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vmdk"},
		exports:             []string{"vmdk"},
		basePartitionTables: defaultBasePartitionTables,
	}

	ovaImgType = imageType{
		name:     "ova",
		filename: "image.ova",
		mimeType: "application/ovf",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: vmdkCommonPackageSet,
		},
		defaultImageConfig:  vmdkDefaultImageConfig,
		kernelOptions:       cloudKernelOptions,
		bootable:            true,
		defaultSize:         2 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vmdk", "ovf", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	containerImgType = imageType{
		name:     "container",
		filename: "container.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: containerPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			NoSElinux:   common.ToPtr(true),
			ExcludeDocs: common.ToPtr(true),
			Locale:      common.ToPtr("C.UTF-8"),
			Timezone:    common.ToPtr("Etc/UTC"),
		},
		image:            containerImage,
		bootable:         false,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "container"},
		exports:          []string{"container"},
	}

	wslImgType = imageType{
		name:     "wsl",
		filename: "wsl.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: containerPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			NoSElinux:   common.ToPtr(true),
			ExcludeDocs: common.ToPtr(true),
			Locale:      common.ToPtr("C.UTF-8"),
			Timezone:    common.ToPtr("Etc/UTC"),
			WSLConfig: &osbuild.WSLConfStageOptions{
				Boot: osbuild.WSLConfBootOptions{
					Systemd: true,
				},
			},
		},
		image:            containerImage,
		bootable:         false,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "container"},
		exports:          []string{"container"},
	}

	minimalrawImgType = imageType{
		name:        "minimal-raw",
		filename:    "disk.raw.xz",
		compression: "xz",
		mimeType:    "application/xz",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: minimalrpmPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: minimalRawServices,
			// NOTE: temporary workaround for a bug in initial-setup that
			// requires a kickstart file in the root directory.
			Files: []*fsnode.File{initialSetupKickstart()},
			Grub2Config: &osbuild.GRUB2Config{
				// Overwrite the default Grub2 timeout value.
				Timeout: 5,
			},
		},
		rpmOstree:           false,
		kernelOptions:       defaultKernelOptions,
		bootable:            true,
		defaultSize:         2 * common.GibiByte,
		image:               diskImage,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "xz"},
		exports:             []string{"xz"},
		basePartitionTables: minimalrawPartitionTables,
	}
)

type distribution struct {
	name               string
	product            string
	osVersion          string
	releaseVersion     string
	modulePlatformID   string
	ostreeRefTmpl      string
	runner             runner.Runner
	arches             map[string]distro.Arch
	defaultImageConfig *distro.ImageConfig
}

// Fedora based OS image configuration defaults
var defaultDistroImageConfig = &distro.ImageConfig{
	Timezone:               common.ToPtr("UTC"),
	Locale:                 common.ToPtr("en_US"),
	DefaultOSCAPDatastream: common.ToPtr(oscap.DefaultFedoraDatastream()),
}

func getISOLabelFunc(variant string) isoLabelFunc {
	const ISO_LABEL = "%s-%s-%s-%s"

	return func(t *imageType) string {
		return fmt.Sprintf(ISO_LABEL, t.Arch().Distro().Product(), t.Arch().Distro().OsVersion(), variant, t.Arch().Name())
	}

}

func getDistro(version int) distribution {
	if version < 0 {
		panic("Invalid Fedora version (must be positive)")
	}
	return distribution{
		name:               fmt.Sprintf("fedora-%d", version),
		product:            "Fedora",
		osVersion:          strconv.Itoa(version),
		releaseVersion:     strconv.Itoa(version),
		modulePlatformID:   fmt.Sprintf("platform:f%d", version),
		ostreeRefTmpl:      fmt.Sprintf("fedora/%d/%%s/iot", version),
		runner:             &runner.Fedora{Version: uint64(version)},
		defaultImageConfig: defaultDistroImageConfig,
	}
}

func (d *distribution) Name() string {
	return d.name
}

func (d *distribution) Codename() string {
	return "" // Fedora does not use distro codename
}

func (d *distribution) Releasever() string {
	return d.releaseVersion
}

func (d *distribution) OsVersion() string {
	return d.releaseVersion
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

func newDistro(version int) distro.Distro {
	rd := getDistro(version)

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

	ociImgType := qcow2ImgType
	ociImgType.name = "oci"

	amiImgType := qcow2ImgType
	amiImgType.name = "ami"
	amiImgType.filename = "image.raw"
	amiImgType.mimeType = "application/octet-stream"
	amiImgType.payloadPipelines = []string{"os", "image"}
	amiImgType.exports = []string{"image"}
	amiImgType.environment = &environment.EC2{}

	openstackImgType := qcow2ImgType
	openstackImgType.name = "openstack"

	vhdImgType := qcow2ImgType
	vhdImgType.name = "vhd"
	vhdImgType.filename = "disk.vhd"
	vhdImgType.mimeType = "application/x-vhd"
	vhdImgType.payloadPipelines = []string{"os", "image", "vpc"}
	vhdImgType.exports = []string{"vpc"}
	vhdImgType.environment = &environment.Azure{}
	vhdImgType.packageSets = map[string]packageSetFunc{
		osPkgsKey: vhdCommonPackageSet,
	}
	vhdConfig := distro.ImageConfig{
		SshdConfig: &osbuild.SshdConfigStageOptions{
			Config: osbuild.SshdConfigConfig{
				ClientAliveInterval: common.ToPtr(120),
			},
		},
	}
	vhdImgType.defaultImageConfig = vhdConfig.InheritFrom(qcow2ImgType.defaultImageConfig)

	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: "fedora",
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
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
			},
		},
		openstackImgType,
	)
	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_VHD,
			},
		},
		vhdImgType,
	)
	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_VMDK,
			},
		},
		vmdkImgType,
	)
	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_OVA,
			},
		},
		ovaImgType,
	)
	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
		},
		amiImgType,
	)
	x86_64.addImageTypes(
		&platform.X86{},
		containerImgType,
		wslImgType,
	)
	x86_64.addImageTypes(
		&platform.X86{
			BasePlatform: platform.BasePlatform{
				FirmwarePackages: []string{
					"biosdevname",
					"iwlwifi-dvm-firmware",
					"iwlwifi-mvm-firmware",
					"microcode_ctl",
				},
			},
			BIOS:       true,
			UEFIVendor: "fedora",
		},
		iotOCIImgType,
		iotCommitImgType,
		iotInstallerImgType,
		imageInstallerImgType,
		liveInstallerImgType,
	)
	x86_64.addImageTypes(
		&platform.X86{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
			BIOS:       false,
			UEFIVendor: "fedora",
		},
		iotRawImgType,
	)
	x86_64.addImageTypes(
		&platform.X86{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
			},
			BIOS:       false,
			UEFIVendor: "fedora",
		},
		iotQcow2ImgType,
	)
	aarch64.addImageTypes(
		&platform.Aarch64{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
		},
		amiImgType,
	)
	aarch64.addImageTypes(
		&platform.Aarch64{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		iotQcow2ImgType,
		ociImgType,
		qcow2ImgType,
	)
	aarch64.addImageTypes(
		&platform.Aarch64{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
			},
		},
		openstackImgType,
	)
	aarch64.addImageTypes(
		&platform.Aarch64{},
		containerImgType,
	)
	aarch64.addImageTypes(
		&platform.Aarch64{
			BasePlatform: platform.BasePlatform{
				FirmwarePackages: []string{
					"arm-image-installer",
					"bcm283x-firmware",
					"brcmfmac-firmware",
					"iwlwifi-mvm-firmware",
					"realtek-firmware",
					"uboot-images-armv8",
				},
			},
			UEFIVendor: "fedora",
		},
		imageInstallerImgType,
		iotCommitImgType,
		iotInstallerImgType,
		iotOCIImgType,
		liveInstallerImgType,
	)
	aarch64.addImageTypes(
		&platform.Aarch64_Fedora{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
			UEFIVendor: "fedora",
			BootFiles: [][2]string{
				{"/usr/lib/ostree-boot/efi/bcm2710-rpi-2-b.dtb", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/bcm2710-rpi-3-b-plus.dtb", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/bcm2710-rpi-3-b.dtb", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/bcm2710-rpi-cm3.dtb", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/bcm2710-rpi-zero-2-w.dtb", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/bcm2710-rpi-zero-2.dtb", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/bcm2711-rpi-4-b.dtb", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/bcm2711-rpi-400.dtb", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/bcm2711-rpi-cm4.dtb", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/bcm2711-rpi-cm4s.dtb", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/bootcode.bin", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/config.txt", "/boot/efi/config.txt"},
				{"/usr/lib/ostree-boot/efi/fixup.dat", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/fixup4.dat", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/fixup4cd.dat", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/fixup4db.dat", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/fixup4x.dat", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/fixup_cd.dat", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/fixup_db.dat", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/fixup_x.dat", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/overlays", "/boot/efi/"},
				{"/usr/share/uboot/rpi_arm64/u-boot.bin", "/boot/efi/rpi-u-boot.bin"},
				{"/usr/lib/ostree-boot/efi/start.elf", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/start4.elf", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/start4cd.elf", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/start4db.elf", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/start4x.elf", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/start_cd.elf", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/start_db.elf", "/boot/efi/"},
				{"/usr/lib/ostree-boot/efi/start_x.elf", "/boot/efi/"},
			},
		},
		iotRawImgType,
	)
	x86_64.addImageTypes(
		&platform.X86{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
		},
		minimalrawImgType,
	)
	aarch64.addImageTypes(
		&platform.Aarch64_Fedora{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
				FirmwarePackages: []string{
					"arm-image-installer",
					"bcm283x-firmware",
					"uboot-images-armv8",
				},
			},
			BootFiles: [][2]string{
				{"/usr/share/uboot/rpi_arm64/u-boot.bin", "/boot/efi/rpi-u-boot.bin"},
			},
		},
		minimalrawImgType,
	)

	x86_64.addImageTypes(
		&platform.X86{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
				FirmwarePackages: []string{
					"grub2-efi-x64",
					"grub2-efi-x64-cdboot",
					"grub2-tools",
					"grub2-tools-minimal",
					"efibootmgr",
					"shim-x64",
					"brcmfmac-firmware",
					"iwlwifi-dvm-firmware",
					"iwlwifi-mvm-firmware",
					"realtek-firmware",
					"microcode_ctl",
				},
			},
			BIOS:       false,
			UEFIVendor: "fedora",
		},
		iotSimplifiedInstallerImgType,
	)

	aarch64.addImageTypes(
		&platform.Aarch64{
			BasePlatform: platform.BasePlatform{
				FirmwarePackages: []string{
					"arm-image-installer",
					"bcm283x-firmware",
					"grub2-efi-aa64",
					"grub2-efi-aa64-cdboot",
					"grub2-tools",
					"grub2-tools-minimal",
					"efibootmgr",
					"shim-aa64",
					"brcmfmac-firmware",
					"iwlwifi-dvm-firmware",
					"iwlwifi-mvm-firmware",
					"realtek-firmware",
					"uboot-images-armv8",
				},
			},
			UEFIVendor: "fedora",
		},
		iotSimplifiedInstallerImgType,
	)

	x86_64.addImageTypes(
		&platform.X86{
			BasePlatform: platform.BasePlatform{
				FirmwarePackages: []string{
					"biosdevname",
					"iwlwifi-dvm-firmware",
					"iwlwifi-mvm-firmware",
					"microcode_ctl",
				},
			},
			BIOS:       true,
			UEFIVendor: "fedora",
		},
		iotBootableContainer,
	)
	aarch64.addImageTypes(
		&platform.Aarch64{
			BasePlatform: platform.BasePlatform{
				FirmwarePackages: []string{
					"arm-image-installer",
					"bcm283x-firmware",
					"brcmfmac-firmware",
					"iwlwifi-mvm-firmware",
					"realtek-firmware",
					"uboot-images-armv8",
				},
			},
			UEFIVendor: "fedora",
		},
		iotBootableContainer,
	)

	ppc64le.addImageTypes(
		&platform.PPC64LE{
			BIOS: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		iotBootableContainer,
	)

	s390x.addImageTypes(
		&platform.S390X{
			Zipl: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		iotBootableContainer,
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
		containerImgType,
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
		containerImgType,
	)

	rd.addArches(x86_64, aarch64, ppc64le, s390x)
	return &rd
}

func ParseID(idStr string) (*distro.ID, error) {
	id, err := distro.ParseID(idStr)
	if err != nil {
		return nil, err
	}

	if id.Name != "fedora" {
		return nil, fmt.Errorf("invalid distro name: %s", id.Name)
	}

	if id.MinorVersion != -1 {
		return nil, fmt.Errorf("fedora distro does not support minor versions")
	}

	return id, nil
}

func DistroFactory(idStr string) distro.Distro {
	id, err := ParseID(idStr)
	if err != nil {
		return nil
	}

	return newDistro(id.MajorVersion)
}
