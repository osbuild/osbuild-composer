package fedora

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/environment"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

const (
	GigaByte = 1024 * 1024 * 1024
	// package set names

	// main/common os image package set name
	osPkgsKey = "packages"

	// container package set name
	containerPkgsKey = "container"

	// installer package set name
	installerPkgsKey = "installer"

	// blueprint package set name
	blueprintPkgsKey = "blueprint"

	// Fedora distribution
	fedora35Distribution = "fedora-35"
	fedora36Distribution = "fedora-36"

	//Kernel options for ami, qcow2, openstack, vhd and vmdk types
	defaultKernelOptions = "ro no_timer_check console=ttyS0,115200n8 biosdevname=0 net.ifnames=0"
)

var (
	mountpointAllowList = []string{
		"/", "/var", "/opt", "/srv", "/usr", "/app", "/data", "/home", "/tmp",
	}

	// Services
	iotServices = []string{
		"NetworkManager.service",
		"firewalld.service",
		"rngd.service",
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

	// Image Definitions
	iotCommitImgType = imageType{
		name:        "fedora-iot-commit",
		nameAliases: []string{"iot-commit"},
		filename:    "commit.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: iotCommitPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: iotServices,
		},
		rpmOstree:        true,
		manifest:         iotCommitManifest,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "ostree-commit", "commit-archive"},
		exports:          []string{"commit-archive"},
	}

	iotOCIImgType = imageType{
		name:        "fedora-iot-container",
		nameAliases: []string{"iot-container"},
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
		},
		rpmOstree:        true,
		bootISO:          false,
		manifest:         iotContainerManifest,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "container-tree", "container"},
		exports:          []string{"container"},
	}

	iotInstallerImgType = imageType{
		name:        "fedora-iot-installer",
		nameAliases: []string{"iot-installer"},
		filename:    "installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			installerPkgsKey: iotInstallerPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale:          "en_US.UTF-8",
			EnabledServices: iotServices,
		},
		rpmOstree:        true,
		bootISO:          true,
		manifest:         iotInstallerManifest,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}

	qcow2ImgType = imageType{
		name:     "qcow2",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: qcow2CommonPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			DefaultTarget: "multi-user.target",
			EnabledServices: []string{
				"cloud-init.service",
				"cloud-config.service",
				"cloud-final.service",
				"cloud-init-local.service",
			},
		},
		kernelOptions:       defaultKernelOptions,
		bootable:            true,
		defaultSize:         2 * GigaByte,
		manifest:            qcow2Manifest,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}

	vhdImgType = imageType{
		name:     "vhd",
		filename: "disk.vhd",
		mimeType: "application/x-vhd",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: vhdCommonPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: "en_US.UTF-8",
			EnabledServices: []string{
				"sshd",
			},
			DefaultTarget: "multi-user.target",
			DisabledServices: []string{
				"proc-sys-fs-binfmt_misc.mount",
				"loadmodules.service",
			},
		},
		kernelOptions:       defaultKernelOptions,
		bootable:            true,
		defaultSize:         2 * GigaByte,
		manifest:            vhdManifest,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vpc"},
		exports:             []string{"vpc"},
		basePartitionTables: defaultBasePartitionTables,
		environment:         &environment.Azure{},
	}

	vmdkImgType = imageType{
		name:     "vmdk",
		filename: "disk.vmdk",
		mimeType: "application/x-vmdk",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: vmdkCommonPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: "en_US.UTF-8",
			EnabledServices: []string{
				"cloud-init.service",
				"cloud-config.service",
				"cloud-final.service",
				"cloud-init-local.service",
			},
		},
		kernelOptions:       defaultKernelOptions,
		bootable:            true,
		defaultSize:         2 * GigaByte,
		manifest:            vmdkManifest,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vmdk"},
		exports:             []string{"vmdk"},
		basePartitionTables: defaultBasePartitionTables,
	}

	openstackImgType = imageType{
		name:     "openstack",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: openstackCommonPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: "en_US.UTF-8",
			EnabledServices: []string{
				"cloud-init.service",
				"cloud-config.service",
				"cloud-final.service",
				"cloud-init-local.service",
			},
		},
		kernelOptions:       defaultKernelOptions,
		bootable:            true,
		defaultSize:         2 * GigaByte,
		manifest:            openstackManifest,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}

	// default EC2 images config (common for all architectures)
	defaultEc2ImageConfig = &distro.ImageConfig{
		DefaultTarget: "multi-user.target",
	}

	amiImgType = imageType{
		name:     "ami",
		filename: "image.raw",
		mimeType: "application/octet-stream",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: ec2CommonPackageSet,
		},
		defaultImageConfig:  defaultEc2ImageConfig,
		kernelOptions:       defaultKernelOptions,
		bootable:            true,
		defaultSize:         6 * GigaByte,
		manifest:            ec2Manifest,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image"},
		exports:             []string{"image"},
		basePartitionTables: defaultBasePartitionTables,
		environment:         &environment.EC2{},
	}

	containerImgType = imageType{
		name:     "container",
		filename: "container.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]packageSetFunc{
			osPkgsKey: containerPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			NoSElinux:   true,
			ExcludeDocs: true,
			Locale:      "C.UTF-8",
			Timezone:    "Etc/UTC",
		},
		manifest:         containerManifest,
		bootable:         false,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "container"},
		exports:          []string{"container"},
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

// Fedora based OS image configuration defaults
var defaultDistroImageConfig = &distro.ImageConfig{
	Timezone: "UTC",
	Locale:   "en_US",
}

// distribution objects without the arches > image types
var distroMap = map[string]distribution{
	fedora35Distribution: {
		name:               fedora35Distribution,
		product:            "Fedora",
		osVersion:          "35",
		releaseVersion:     "35",
		modulePlatformID:   "platform:f35",
		vendor:             "fedora",
		ostreeRefTmpl:      "fedora/35/%s/iot",
		isolabelTmpl:       "Fedora-35-BaseOS-%s",
		runner:             &runner.Fedora{Version: 35},
		defaultImageConfig: defaultDistroImageConfig,
	},
	fedora36Distribution: {
		name:               fedora36Distribution,
		product:            "Fedora",
		osVersion:          "36",
		releaseVersion:     "36",
		modulePlatformID:   "platform:f36",
		vendor:             "fedora",
		ostreeRefTmpl:      "fedora/36/%s/iot",
		isolabelTmpl:       "Fedora-36-BaseOS-%s",
		runner:             &runner.Fedora{Version: 36},
		defaultImageConfig: defaultDistroImageConfig,
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

type manifestFunc func(m *manifest.Manifest, workload workload.Workload, t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSets map[string]rpmmd.PackageSet, rng *rand.Rand) error

type packageSetFunc func(t *imageType) rpmmd.PackageSet

type imageType struct {
	arch               *architecture
	platform           platform.Platform
	environment        environment.Environment
	name               string
	nameAliases        []string
	filename           string
	mimeType           string
	packageSets        map[string]packageSetFunc
	defaultImageConfig *distro.ImageConfig
	kernelOptions      string
	defaultSize        uint64
	buildPipelines     []string
	payloadPipelines   []string
	exports            []string
	manifest           manifestFunc

	// bootISO: installable ISO
	bootISO bool
	// rpmOstree: iot/ostree
	rpmOstree bool
	// bootable image
	bootable bool
	// List of valid arches for the image type
	basePartitionTables distro.BasePartitionTableMap
}

func (t *imageType) Name() string {
	return t.name
}

func (t *imageType) Arch() distro.Arch {
	return t.arch
}

func (t *imageType) Filename() string {
	return t.filename
}

func (t *imageType) MIMEType() string {
	return t.mimeType
}

func (t *imageType) OSTreeRef() string {
	d := t.arch.distro
	if t.rpmOstree {
		return fmt.Sprintf(d.ostreeRefTmpl, t.arch.Name())
	}
	return ""
}

func (t *imageType) Size(size uint64) uint64 {
	const MegaByte = 1024 * 1024
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.name == "vhd" && size%MegaByte != 0 {
		size = (size/MegaByte + 1) * MegaByte
	}
	if size == 0 {
		size = t.defaultSize
	}
	return size
}

func (t *imageType) PackageSets(bp blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig) map[string][]rpmmd.PackageSet {
	// merge package sets that appear in the image type with the package sets
	// of the same name from the distro and arch
	packageSets := make(map[string]rpmmd.PackageSet)

	for name, getter := range t.packageSets {
		packageSets[name] = getter(t)
	}

	// amend with repository information
	globalRepos := make([]rpmmd.RepoConfig, 0)
	for _, repo := range repos {
		if len(repo.PackageSets) > 0 {
			// only apply the repo to the listed package sets
			for _, psName := range repo.PackageSets {
				ps := packageSets[psName]
				ps.Repositories = append(ps.Repositories, repo)
				packageSets[psName] = ps
			}
		} else {
			// no package sets were listed, so apply the repo
			// to all package sets
			globalRepos = append(globalRepos, repo)
		}
	}

	// create a manifest object and instantiate it with the computed packageSetChains
	manifest, err := t.initializeManifest(&bp, options, globalRepos, packageSets, nil, 0)
	if err != nil {
		// TODO: handle manifest initialization errors more gracefully, we
		// refuse to initialize manifests with invalid config.
		return nil
	}

	return manifest.GetPackageSetChains()
}

func (t *imageType) BuildPipelines() []string {
	return t.buildPipelines
}

func (t *imageType) PayloadPipelines() []string {
	return t.payloadPipelines
}

func (t *imageType) PayloadPackageSets() []string {
	return []string{blueprintPkgsKey}
}

func (t *imageType) PackageSetsChains() map[string][]string {
	return make(map[string][]string)
}

func (t *imageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	return []string{"assembler"}
}

func (t *imageType) getPartitionTable(
	mountpoints []blueprint.FilesystemCustomization,
	options distro.ImageOptions,
	rng *rand.Rand,
) (*disk.PartitionTable, error) {
	basePartitionTable, exists := t.basePartitionTables[t.arch.Name()]
	if !exists {
		return nil, fmt.Errorf("unknown arch: " + t.arch.Name())
	}

	imageSize := t.Size(options.Size)

	lvmify := !t.rpmOstree

	return disk.NewPartitionTable(&basePartitionTable, mountpoints, imageSize, lvmify, rng)
}

func (t *imageType) getDefaultImageConfig() *distro.ImageConfig {
	// ensure that image always returns non-nil default config
	imageConfig := t.defaultImageConfig
	if imageConfig == nil {
		imageConfig = &distro.ImageConfig{}
	}
	return imageConfig.InheritFrom(t.arch.distro.getDefaultImageConfig())

}

func (t *imageType) PartitionType() string {
	basePartitionTable, exists := t.basePartitionTables[t.arch.Name()]
	if !exists {
		return ""
	}

	return basePartitionTable.Type
}

func (t *imageType) initializeManifest(bp *blueprint.Blueprint,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.Spec,
	seed int64) (*manifest.Manifest, error) {

	if err := t.checkOptions(bp.Customizations, options, containers); err != nil {
		return nil, err
	}

	// TODO: let image types specify valid workloads, rather than
	// always assume Custom.
	w := &workload.Custom{
		BaseWorkload: workload.BaseWorkload{
			Repos: packageSets[blueprintPkgsKey].Repositories,
		},
		Packages: bp.GetPackagesEx(false),
	}
	if services := bp.Customizations.GetServices(); services != nil {
		w.Services = services.Enabled
		w.DisabledServices = services.Disabled
	}

	source := rand.NewSource(seed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	manifest := manifest.New()
	err := t.manifest(&manifest, w, t, bp.Customizations, options, repos, packageSets, rng)
	if err != nil {
		return nil, err
	}

	return &manifest, nil
}

func (t *imageType) Manifest(customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string][]rpmmd.PackageSpec,
	containers []container.Spec,
	seed int64) (distro.Manifest, error) {

	bp := &blueprint.Blueprint{}
	err := bp.Initialize()
	if err != nil {
		panic("could not initialize empty blueprint: " + err.Error())
	}
	bp.Customizations = customizations

	manifest, err := t.initializeManifest(bp, options, repos, nil, containers, seed)
	if err != nil {
		return distro.Manifest{}, err
	}

	return manifest.Serialize(packageSets)
}

// checkOptions checks the validity and compatibility of options and customizations for the image type.
func (t *imageType) checkOptions(customizations *blueprint.Customizations, options distro.ImageOptions, containers []container.Spec) error {

	if len(containers) > 0 {
		return fmt.Errorf("embedding containers is not supported for %s on %s", t.name, t.arch.distro.name)
	}

	if t.bootISO && t.rpmOstree {
		if options.OSTree.Parent == "" {
			return fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.name)
		}

		if t.name == "iot-installer" || t.name == "fedora-iot-installer" {
			allowed := []string{"User", "Group"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
			}
		}
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.rpmOstree {
		return fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()

	if mountpoints != nil && t.rpmOstree {
		return fmt.Errorf("Custom mountpoints are not supported for ostree types")
	}

	invalidMountpoints := []string{}
	for _, m := range mountpoints {
		if !distro.IsMountpointAllowed(m.Mountpoint, mountpointAllowList) {
			invalidMountpoints = append(invalidMountpoints, m.Mountpoint)
		}
	}

	if len(invalidMountpoints) > 0 {
		return fmt.Errorf("The following custom mountpoints are not supported %+q", invalidMountpoints)
	}

	return nil
}

func NewHostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	return newDistro(name)
}

// New creates a new distro object, defining the supported architectures and image types
func NewF35() distro.Distro {
	return newDistro(fedora35Distribution)
}
func NewF36() distro.Distro {
	return newDistro(fedora36Distribution)
}

func newDistro(distroName string) distro.Distro {

	rd := distroMap[distroName]

	// Architecture definitions
	x86_64 := architecture{
		name:   distro.X86_64ArchName,
		distro: &rd,
	}

	aarch64 := architecture{
		name:   distro.Aarch64ArchName,
		distro: &rd,
	}

	s390x := architecture{
		distro: &rd,
		name:   distro.S390xArchName,
	}

	ociImgType := qcow2ImgType
	ociImgType.name = "oci"

	x86_64.addImageTypes(
		&platform.X86{
			BIOS:       true,
			UEFIVendor: "fedora",
		},
		qcow2ImgType,
		openstackImgType,
		vhdImgType,
		vmdkImgType,
		ociImgType,
	)
	x86_64.addImageTypes(
		&platform.X86{
			BIOS: true,
		},
		amiImgType,
	)
	x86_64.addImageTypes(
		&platform.X86{},
		containerImgType,
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
					"iwl6000-firmware",
					"iwl6050-firmware",
				},
			},
			BIOS:       true,
			UEFIVendor: "fedora",
		},
		iotOCIImgType,
		iotCommitImgType,
		iotInstallerImgType,
	)
	aarch64.addImageTypes(
		&platform.Aarch64{
			UEFIVendor: "fedora",
		},
		amiImgType,
		qcow2ImgType,
		openstackImgType,
		ociImgType,
	)
	aarch64.addImageTypes(
		&platform.Aarch64{},
		containerImgType,
	)
	aarch64.addImageTypes(
		&platform.Aarch64{
			BasePlatform: platform.BasePlatform{
				FirmwarePackages: []string{
					"uboot-images-armv8", // ??
					"bcm283x-firmware",
					"arm-image-installer", // ??
				},
			},
			UEFIVendor: "fedora",
		},
		iotCommitImgType,
		iotOCIImgType,
		iotInstallerImgType,
	)

	s390x.addImageTypes(nil)

	rd.addArches(x86_64, aarch64, s390x)
	return &rd
}
