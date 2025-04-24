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
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
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
)

var (
	oscapProfileAllowList = []oscap.Profile{
		oscap.Ospp,
		oscap.PciDss,
		oscap.Standard,
	}

	// Default directory size minimums for all image types.
	requiredDirectorySizes = map[string]uint64{
		"/":    1 * datasizes.GiB,
		"/usr": 2 * datasizes.GiB,
	}
)

// kernel command line arguments
// NOTE: we define them as functions to make sure they globals are never
// modified

// Default kernel command line
func defaultKernelOptions() []string { return []string{"ro"} }

// Added kernel command line options for ami, qcow2, openstack, vhd and vmdk types
func cloudKernelOptions() []string {
	return []string{"ro", "no_timer_check", "console=ttyS0,115200n8", "biosdevname=0", "net.ifnames=0"}
}

// Added kernel command line options for iot-raw-image and iot-qcow2-image types
func ostreeDeploymentKernelOptions() []string {
	return []string{"modprobe.blacklist=vc4", "rw", "coreos.no_persist_ip"}
}

// Image Definitions
func mkImageInstallerImgType(d distribution) imageType {
	return imageType{
		name:        "minimal-installer",
		nameAliases: []string{"image-installer", "fedora-image-installer"},
		filename:    "installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: func(t *imageType) (rpmmd.PackageSet, error) {
				// use the minimal raw image type for the OS package set
				return defs.PackageSet(t, "minimal-raw-xz", VersionReplacements())
			},
			installerPkgsKey: packageSetLoader,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: common.ToPtr("en_US.UTF-8"),
		},
		bootable:  true,
		bootISO:   true,
		rpmOstree: false,
		image:     imageInstallerImage,
		// We don't know the variant of the OS pipeline being installed
		isoLabel:               getISOLabelFunc("Unknown"),
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"anaconda-tree", "efiboot-tree", "os", "bootiso-tree", "bootiso"},
		exports:                []string{"bootiso"},
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

func mkLiveInstallerImgType(d distribution) imageType {
	return imageType{
		name:        "workstation-live-installer",
		nameAliases: []string{"live-installer"},
		filename:    "live-installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			installerPkgsKey: packageSetLoader,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: common.ToPtr("en_US.UTF-8"),
		},
		bootable:               true,
		bootISO:                true,
		rpmOstree:              false,
		image:                  liveInstallerImage,
		isoLabel:               getISOLabelFunc("Workstation"),
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"anaconda-tree", "efiboot-tree", "bootiso-tree", "bootiso"},
		exports:                []string{"bootiso"},
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

func mkIotCommitImgType(d distribution) imageType {
	return imageType{
		name:        "iot-commit",
		nameAliases: []string{"fedora-iot-commit"},
		filename:    "commit.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: packageSetLoader,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices:        iotServicesForVersion(&d),
			DracutConf:             []*osbuild.DracutConfStageOptions{osbuild.FIPSDracutConfStageOptions},
			MachineIdUninitialized: common.ToPtr(false),
		},
		rpmOstree:              true,
		image:                  iotCommitImage,
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"os", "ostree-commit", "commit-archive"},
		exports:                []string{"commit-archive"},
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

func mkIotBootableContainer(d distribution) imageType {
	return imageType{
		name:     "iot-bootable-container",
		filename: "iot-bootable-container.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: packageSetLoader,
		},
		defaultImageConfig: &distro.ImageConfig{
			MachineIdUninitialized: common.ToPtr(false),
		},
		rpmOstree:              true,
		image:                  bootableContainerImage,
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"os", "ostree-commit", "ostree-encapsulate"},
		exports:                []string{"ostree-encapsulate"},
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

func mkIotOCIImgType(d distribution) imageType {
	return imageType{
		name:        "iot-container",
		nameAliases: []string{"fedora-iot-container"},
		filename:    "container.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: packageSetLoader,
			containerPkgsKey: func(t *imageType) (rpmmd.PackageSet, error) {
				return rpmmd.PackageSet{}, nil
			},
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices:        iotServicesForVersion(&d),
			DracutConf:             []*osbuild.DracutConfStageOptions{osbuild.FIPSDracutConfStageOptions},
			MachineIdUninitialized: common.ToPtr(false),
		},
		rpmOstree:              true,
		bootISO:                false,
		image:                  iotContainerImage,
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"os", "ostree-commit", "container-tree", "container"},
		exports:                []string{"container"},
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

func mkIotInstallerImgType(d distribution) imageType {
	return imageType{
		name:        "iot-installer",
		nameAliases: []string{"fedora-iot-installer"},
		filename:    "installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			installerPkgsKey: packageSetLoader,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: iotServicesForVersion(&d),
			Locale:          common.ToPtr("en_US.UTF-8"),
		},
		rpmOstree:              true,
		bootISO:                true,
		image:                  iotInstallerImage,
		isoLabel:               getISOLabelFunc("IoT"),
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"anaconda-tree", "efiboot-tree", "bootiso-tree", "bootiso"},
		exports:                []string{"bootiso"},
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

func mkIotSimplifiedInstallerImgType(d distribution) imageType {
	return imageType{
		name:     "iot-simplified-installer",
		filename: "simplified-installer.iso",
		mimeType: "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			installerPkgsKey: packageSetLoader,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: iotServicesForVersion(&d),
			Keyboard: &osbuild.KeymapStageOptions{
				Keymap: "us",
			},
			Locale:                    common.ToPtr("C.UTF-8"),
			OSTreeConfSysrootReadOnly: common.ToPtr(true),
			LockRootUser:              common.ToPtr(true),
			IgnitionPlatform:          common.ToPtr("metal"),
		},
		defaultSize:            10 * datasizes.GibiByte,
		rpmOstree:              true,
		bootable:               true,
		bootISO:                true,
		image:                  iotSimplifiedInstallerImage,
		isoLabel:               getISOLabelFunc("IoT"),
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"ostree-deployment", "image", "xz", "coi-tree", "efiboot-tree", "bootiso-tree", "bootiso"},
		exports:                []string{"bootiso"},
		kernelOptions:          ostreeDeploymentKernelOptions(),
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

func mkIotRawImgType(d distribution) imageType {
	return imageType{
		name:        "iot-raw-xz",
		nameAliases: []string{"iot-raw-image", "fedora-iot-raw-image"},
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
		defaultSize:      4 * datasizes.GibiByte,
		rpmOstree:        true,
		bootable:         true,
		image:            iotImage,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-deployment", "image", "xz"},
		exports:          []string{"xz"},
		kernelOptions:    ostreeDeploymentKernelOptions(),

		// Passing an empty map into the required partition sizes disables the
		// default partition sizes normally set so our `basePartitionTables` can
		// override them (and make them smaller, in this case).
		requiredPartitionSizes: map[string]uint64{},
	}
}

func mkIotQcow2ImgType(d distribution) imageType {
	return imageType{
		name:        "iot-qcow2",
		nameAliases: []string{"iot-qcow2-image"}, // kept for backwards compatibility
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
		defaultSize:            10 * datasizes.GibiByte,
		rpmOstree:              true,
		bootable:               true,
		image:                  iotImage,
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"ostree-deployment", "image", "qcow2"},
		exports:                []string{"qcow2"},
		kernelOptions:          ostreeDeploymentKernelOptions(),
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

func mkQcow2ImgType(d distribution) imageType {
	return imageType{
		name:        "server-qcow2",
		nameAliases: []string{"qcow2"}, // kept for backwards compatibility
		filename:    "disk.qcow2",
		mimeType:    "application/x-qemu-disk",
		environment: &environment.KVM{},
		packageSets: map[string]packageSetFunc{
			osPkgsKey: packageSetLoader,
		},
		defaultImageConfig: &distro.ImageConfig{
			DefaultTarget: common.ToPtr("multi-user.target"),
		},
		kernelOptions:          cloudKernelOptions(),
		bootable:               true,
		defaultSize:            5 * datasizes.GibiByte,
		image:                  diskImage,
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"os", "image", "qcow2"},
		exports:                []string{"qcow2"},
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

var (
	vmdkDefaultImageConfig = &distro.ImageConfig{
		Locale: common.ToPtr("en_US.UTF-8"),
		EnabledServices: []string{
			"cloud-init.service",
			"cloud-config.service",
			"cloud-final.service",
			"cloud-init-local.service",
		},
	}
)

func mkVmdkImgType(d distribution) imageType {
	return imageType{
		name:        "server-vmdk",
		nameAliases: []string{"vmdk"}, // kept for backwards compatibility
		filename:    "disk.vmdk",
		mimeType:    "application/x-vmdk",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: packageSetLoader,
		},
		defaultImageConfig:     vmdkDefaultImageConfig,
		kernelOptions:          cloudKernelOptions(),
		bootable:               true,
		defaultSize:            2 * datasizes.GibiByte,
		image:                  diskImage,
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"os", "image", "vmdk"},
		exports:                []string{"vmdk"},
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

func mkOvaImgType(d distribution) imageType {
	return imageType{
		name:        "server-ova",
		nameAliases: []string{"ova"}, // kept for backwards compatibility
		filename:    "image.ova",
		mimeType:    "application/ovf",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: packageSetLoader,
		},
		defaultImageConfig:     vmdkDefaultImageConfig,
		kernelOptions:          cloudKernelOptions(),
		bootable:               true,
		defaultSize:            2 * datasizes.GibiByte,
		image:                  diskImage,
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"os", "image", "vmdk", "ovf", "archive"},
		exports:                []string{"archive"},
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

func mkContainerImgType(d distribution) imageType {
	return imageType{
		name:     "container",
		filename: "container.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: packageSetLoader,
		},
		defaultImageConfig: &distro.ImageConfig{
			NoSElinux:   common.ToPtr(true),
			ExcludeDocs: common.ToPtr(true),
			Locale:      common.ToPtr("C.UTF-8"),
			Timezone:    common.ToPtr("Etc/UTC"),
		},
		image:                  containerImage,
		bootable:               false,
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"os", "container"},
		exports:                []string{"container"},
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

func mkWslImgType(d distribution) imageType {
	return imageType{
		name:        "wsl",
		nameAliases: []string{"server-wsl"}, // this is the eventual name, and `wsl` the alias but we've been having issues with CI renaming it
		filename:    "wsl.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: packageSetLoader,
		},
		defaultImageConfig: &distro.ImageConfig{
			CloudInit: []*osbuild.CloudInitStageOptions{
				{
					Filename: "99_wsl.cfg",
					Config: osbuild.CloudInitConfigFile{
						DatasourceList: []string{
							"WSL",
							"None",
						},
						Network: &osbuild.CloudInitConfigNetwork{
							Config: "disabled",
						},
					},
				},
			},
			NoSElinux:   common.ToPtr(true),
			ExcludeDocs: common.ToPtr(true),
			Locale:      common.ToPtr("C.UTF-8"),
			Timezone:    common.ToPtr("Etc/UTC"),
			WSLConfig: &distro.WSLConfig{
				BootSystemd: true,
			},
		},
		image:                  containerImage,
		bootable:               false,
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"os", "container"},
		exports:                []string{"container"},
		requiredPartitionSizes: requiredDirectorySizes,
	}
}

func mkMinimalRawImgType(d distribution) imageType {
	it := imageType{
		name:        "minimal-raw-xz",
		nameAliases: []string{"minimal-raw"}, // kept for backwards compatibility
		filename:    "disk.raw.xz",
		compression: "xz",
		mimeType:    "application/xz",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: packageSetLoader,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: minimalServicesForVersion(&d),
			// NOTE: temporary workaround for a bug in initial-setup that
			// requires a kickstart file in the root directory.
			Files: []*fsnode.File{initialSetupKickstart()},
			Grub2Config: &osbuild.GRUB2Config{
				// Overwrite the default Grub2 timeout value.
				Timeout: 5,
			},
			InstallWeakDeps: common.ToPtr(common.VersionLessThan(d.osVersion, VERSION_MINIMAL_WEAKDEPS)),
		},
		rpmOstree:              false,
		kernelOptions:          defaultKernelOptions(),
		bootable:               true,
		defaultSize:            2 * datasizes.GibiByte,
		image:                  diskImage,
		buildPipelines:         []string{"build"},
		payloadPipelines:       []string{"os", "image", "xz"},
		exports:                []string{"xz"},
		requiredPartitionSizes: requiredDirectorySizes,
	}
	if common.VersionGreaterThanOrEqual(d.osVersion, "43") {
		// from Fedora 43 onward, we stop writing /etc/fstab and start using
		// mount units only
		it.defaultImageConfig.MountUnits = common.ToPtr(true)

		// when using systemd mount units we also want them to be mounted rw
		// while the default options are not
		it.kernelOptions = []string{"rw"}
	}
	return it
}

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

func defaultDistroInstallerConfig(d *distribution) *distro.InstallerConfig {
	config := distro.InstallerConfig{}
	// In Fedora 42 the ifcfg module was replaced by net-lib.
	if common.VersionLessThan(d.osVersion, "42") {
		config.AdditionalDracutModules = append(config.AdditionalDracutModules, "ifcfg")
	} else {
		config.AdditionalDracutModules = append(config.AdditionalDracutModules, "net-lib")
	}

	return &config
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
	nameVer := fmt.Sprintf("fedora-%d", version)
	return distribution{
		name:               nameVer,
		product:            "Fedora",
		osVersion:          strconv.Itoa(version),
		releaseVersion:     strconv.Itoa(version),
		modulePlatformID:   fmt.Sprintf("platform:f%d", version),
		ostreeRefTmpl:      fmt.Sprintf("fedora/%d/%%s/iot", version),
		runner:             &runner.Fedora{Version: uint64(version)},
		defaultImageConfig: common.Must(defs.DistroImageConfig(nameVer)),
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

	riscv64 := architecture{
		name:   arch.ARCH_RISCV64.String(),
		distro: &rd,
	}

	qcow2ImgType := mkQcow2ImgType(rd)

	ociImgType := qcow2ImgType
	ociImgType.name = "server-oci"
	ociImgType.nameAliases = []string{"oci"} // kept for backwards compatibility

	amiImgType := qcow2ImgType
	amiImgType.name = "server-ami"
	amiImgType.nameAliases = []string{"ami"} // kept for backwards compatibility
	amiImgType.filename = "image.raw"
	amiImgType.mimeType = "application/octet-stream"
	amiImgType.payloadPipelines = []string{"os", "image"}
	amiImgType.exports = []string{"image"}
	amiImgType.environment = &environment.EC2{}

	openstackImgType := qcow2ImgType
	openstackImgType.name = "server-openstack"
	openstackImgType.nameAliases = []string{"openstack"} // kept for backwards compatibility

	vhdImgType := qcow2ImgType
	vhdImgType.name = "server-vhd"
	vhdImgType.nameAliases = []string{"vhd"} // kept for backwards compatibility
	vhdImgType.filename = "disk.vhd"
	vhdImgType.mimeType = "application/x-vhd"
	vhdImgType.payloadPipelines = []string{"os", "image", "vpc"}
	vhdImgType.exports = []string{"vpc"}
	vhdImgType.environment = &environment.Azure{}
	vhdImgType.packageSets = map[string]packageSetFunc{
		osPkgsKey: packageSetLoader,
	}
	vhdConfig := distro.ImageConfig{
		SshdConfig: &osbuild.SshdConfigStageOptions{
			Config: osbuild.SshdConfigConfig{
				ClientAliveInterval: common.ToPtr(120),
			},
		},
	}
	vhdImgType.defaultImageConfig = vhdConfig.InheritFrom(qcow2ImgType.defaultImageConfig)

	minimalrawZstdImgType := mkMinimalRawImgType(rd)
	minimalrawZstdImgType.name = "minimal-raw-zst"
	minimalrawZstdImgType.nameAliases = []string{}
	minimalrawZstdImgType.filename = "disk.raw.zst"
	minimalrawZstdImgType.mimeType = "application/zstd"
	minimalrawZstdImgType.compression = "zstd"
	minimalrawZstdImgType.payloadPipelines = []string{"os", "image", "zstd"}
	minimalrawZstdImgType.exports = []string{"zstd"}

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
		mkVmdkImgType(rd),
	)
	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_OVA,
			},
		},
		mkOvaImgType(rd),
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
		mkContainerImgType(rd),
		mkWslImgType(rd),
	)

	// add distro installer configuration to all installer types
	distroInstallerConfig := defaultDistroInstallerConfig(&rd)

	liveInstallerImgType := mkLiveInstallerImgType(rd)
	liveInstallerImgType.defaultInstallerConfig = distroInstallerConfig

	imageInstallerImgType := mkImageInstallerImgType(rd)
	imageInstallerImgType.defaultInstallerConfig = distroInstallerConfig

	iotInstallerImgType := mkIotInstallerImgType(rd)
	iotInstallerImgType.defaultInstallerConfig = distroInstallerConfig

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
		mkIotOCIImgType(rd),
		mkIotCommitImgType(rd),
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
		mkIotRawImgType(rd),
	)
	x86_64.addImageTypes(
		&platform.X86{
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
			},
			BIOS:       false,
			UEFIVendor: "fedora",
		},
		mkIotQcow2ImgType(rd),
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
		mkIotQcow2ImgType(rd),
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
		mkContainerImgType(rd),
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
		mkIotCommitImgType(rd),
		iotInstallerImgType,
		mkIotOCIImgType(rd),
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
		mkIotRawImgType(rd),
	)
	x86_64.addImageTypes(
		&platform.X86{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
		},
		mkMinimalRawImgType(rd),
		minimalrawZstdImgType,
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
		mkMinimalRawImgType(rd),
		minimalrawZstdImgType,
	)

	iotSimplifiedInstallerImgType := mkIotSimplifiedInstallerImgType(rd)
	iotSimplifiedInstallerImgType.defaultInstallerConfig = distroInstallerConfig

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
		mkIotBootableContainer(rd),
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
		mkIotBootableContainer(rd),
	)

	ppc64le.addImageTypes(
		&platform.PPC64LE{
			BIOS: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		mkIotBootableContainer(rd),
	)

	s390x.addImageTypes(
		&platform.S390X{
			Zipl: true,
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_QCOW2,
				QCOW2Compat: "1.1",
			},
		},
		mkIotBootableContainer(rd),
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
		mkContainerImgType(rd),
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
		mkContainerImgType(rd),
	)

	// XXX: there is no "qcow2" for riscv64 yet because there is
	// no "@Fedora Cloud Server" group
	riscv64.addImageTypes(
		&platform.RISCV64{},
		mkContainerImgType(rd),
	)
	riscv64.addImageTypes(
		&platform.RISCV64{
			UEFIVendor: "fedora",
			BasePlatform: platform.BasePlatform{
				ImageFormat: platform.FORMAT_RAW,
			},
		},
		mkMinimalRawImgType(rd),
		minimalrawZstdImgType,
	)

	rd.addArches(x86_64, aarch64, ppc64le, s390x, riscv64)
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

func iotServicesForVersion(d *distribution) []string {
	services := []string{
		"NetworkManager.service",
		"firewalld.service",
		"sshd.service",
		"greenboot-grub2-set-counter",
		"greenboot-grub2-set-success",
		"greenboot-healthcheck",
		"greenboot-rpm-ostree-grub2-check-fallback",
		"greenboot-status",
		"greenboot-task-runner",
		"redboot-auto-reboot",
		"redboot-task-runner",
	}

	if common.VersionLessThan(d.osVersion, "42") {
		services = append(services, []string{
			"zezere_ignition.timer",
			"zezere_ignition_banner.service",
			"parsec",
			"dbus-parsec",
		}...)
	}

	return services
}

func minimalServicesForVersion(d *distribution) []string {
	services := []string{
		"NetworkManager.service",
		"initial-setup.service",
		"sshd.service",
	}

	if common.VersionLessThan(d.osVersion, "43") {
		services = append(services, []string{"firewalld.service"}...)
	}

	return services
}
