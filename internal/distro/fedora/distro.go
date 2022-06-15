package fedora

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"path"
	"sort"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const (
	GigaByte = 1024 * 1024 * 1024
	// package set names

	// build package set name
	buildPkgsKey = "build"

	// main/common os image package set name
	osPkgsKey = "packages"

	// container package set name
	containerPkgsKey = "container"

	// installer package set name
	installerPkgsKey = "installer"

	// blueprint package set name
	blueprintPkgsKey = "blueprint"

	// Fedora distribution
	fedora34Distribution = "fedora-34"
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
			buildPkgsKey: iotBuildPackageSet,
			osPkgsKey:    iotCommitPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: iotServices,
		},
		rpmOstree:        true,
		pipelines:        iotCommitPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "commit-archive"},
		exports:          []string{"commit-archive"},
	}

	iotOCIImgType = imageType{
		name:        "fedora-iot-container",
		nameAliases: []string{"iot-container"},
		filename:    "container.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: iotBuildPackageSet,
			osPkgsKey:    iotCommitPackageSet,
			containerPkgsKey: func(t *imageType) rpmmd.PackageSet {
				return rpmmd.PackageSet{
					Include: []string{"nginx"},
				}
			},
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: iotServices,
		},
		rpmOstree:        true,
		bootISO:          false,
		pipelines:        iotContainerPipelines,
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
			buildPkgsKey:     iotInstallerBuildPackageSet,
			osPkgsKey:        iotCommitPackageSet,
			installerPkgsKey: iotInstallerPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale:          "en_US.UTF-8",
			EnabledServices: iotServices,
		},
		rpmOstree:        true,
		bootISO:          true,
		pipelines:        iotInstallerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}

	qcow2ImgType = imageType{
		name:     "qcow2",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    qcow2CommonPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
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
		pipelines:           qcow2Pipelines,
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
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    vhdCommonPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: "en_US.UTF-8",
			EnabledServices: []string{
				"sshd",
				"waagent",
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
		pipelines:           vhdPipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vpc"},
		exports:             []string{"vpc"},
		basePartitionTables: defaultBasePartitionTables,
	}

	vmdkImgType = imageType{
		name:     "vmdk",
		filename: "disk.vmdk",
		mimeType: "application/x-vmdk",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    vmdkCommonPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
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
		pipelines:           vmdkPipelines,
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
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    openstackCommonPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
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
		pipelines:           openstackPipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}

	// default EC2 images config (common for all architectures)
	defaultEc2ImageConfig = &distro.ImageConfig{
		EnabledServices: []string{
			"cloud-init.service",
		},
		DefaultTarget: "multi-user.target",
	}

	amiImgType = imageType{
		name:     "ami",
		filename: "image.raw",
		mimeType: "application/octet-stream",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: ec2BuildPackageSet,
			osPkgsKey:    ec2CommonPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultEc2ImageConfig,
		kernelOptions:       defaultKernelOptions,
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         6 * GigaByte,
		pipelines:           ec2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image"},
		exports:             []string{"image"},
		basePartitionTables: defaultBasePartitionTables,
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
	runner             string
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
	fedora34Distribution: {
		name:               fedora34Distribution,
		product:            "Fedora",
		osVersion:          "34",
		releaseVersion:     "34",
		modulePlatformID:   "platform:f34",
		vendor:             "fedora",
		ostreeRefTmpl:      "fedora/34/%s/iot",
		isolabelTmpl:       "Fedora-34-BaseOS-%s",
		runner:             "org.osbuild.fedora34",
		defaultImageConfig: defaultDistroImageConfig,
	},
	fedora35Distribution: {
		name:               fedora35Distribution,
		product:            "Fedora",
		osVersion:          "35",
		releaseVersion:     "35",
		modulePlatformID:   "platform:f35",
		vendor:             "fedora",
		ostreeRefTmpl:      "fedora/35/%s/iot",
		isolabelTmpl:       "Fedora-35-BaseOS-%s",
		runner:             "org.osbuild.fedora35",
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
		runner:             "org.osbuild.fedora36",
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
	legacy           string
	bootType         distro.BootType
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

func (a *architecture) addImageTypes(imageTypes ...imageType) {
	if a.imageTypes == nil {
		a.imageTypes = map[string]distro.ImageType{}
	}
	for idx := range imageTypes {
		it := imageTypes[idx]
		it.arch = a
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

type pipelinesFunc func(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error)

type packageSetFunc func(t *imageType) rpmmd.PackageSet

type imageType struct {
	arch               *architecture
	name               string
	nameAliases        []string
	filename           string
	mimeType           string
	packageSets        map[string]packageSetFunc
	packageSetChains   map[string][]string
	defaultImageConfig *distro.ImageConfig
	kernelOptions      string
	defaultSize        uint64
	buildPipelines     []string
	payloadPipelines   []string
	exports            []string
	pipelines          pipelinesFunc

	// bootISO: installable ISO
	bootISO bool
	// rpmOstree: iot/ostree
	rpmOstree bool
	// bootable image
	bootable bool
	// If set to a value, it is preferred over the architecture value
	bootType distro.BootType
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

func (t *imageType) getPackages(name string) rpmmd.PackageSet {
	getter := t.packageSets[name]
	if getter == nil {
		return rpmmd.PackageSet{}
	}

	return getter(t)
}

func (t *imageType) PackageSets(bp blueprint.Blueprint, repos []rpmmd.RepoConfig) map[string][]rpmmd.PackageSet {
	// merge package sets that appear in the image type with the package sets
	// of the same name from the distro and arch
	mergedSets := make(map[string]rpmmd.PackageSet)

	imageSets := t.packageSets

	for name := range imageSets {
		mergedSets[name] = t.getPackages(name)
	}

	if _, hasPackages := imageSets[osPkgsKey]; !hasPackages {
		// should this be possible??
		mergedSets[osPkgsKey] = rpmmd.PackageSet{}
	}

	// every image type must define a 'build' package set
	if _, hasBuild := imageSets[buildPkgsKey]; !hasBuild {
		panic(fmt.Sprintf("'%s' image type has no '%s' package set defined", t.name, buildPkgsKey))
	}

	// blueprint packages
	bpPackages := bp.GetPackages()
	timezone, _ := bp.Customizations.GetTimezoneSettings()
	if timezone != nil {
		bpPackages = append(bpPackages, "chrony")
	}

	// if we have file system customization that will need to a new mount point
	// the layout is converted to LVM so we need to corresponding packages
	if !t.rpmOstree {

		pt, exists := t.basePartitionTables[t.arch.Name()]
		if !exists {
			panic(fmt.Sprintf("unknown architecture with boot type: %s %s", t.arch.Name(), t.bootType))
		}
		haveNewMountpoint := false

		if fs := bp.Customizations.GetFilesystems(); fs != nil {
			for i := 0; !haveNewMountpoint && i < len(fs); i++ {
				haveNewMountpoint = !pt.ContainsMountpoint(fs[i].Mountpoint)
			}
		}

		if haveNewMountpoint {
			bpPackages = append(bpPackages, "lvm2")
		}
	}

	// depsolve bp packages separately
	// bp packages aren't restricted by exclude lists
	mergedSets[blueprintPkgsKey] = rpmmd.PackageSet{Include: bpPackages}
	kernel := bp.Customizations.GetKernel().Name

	// add bp kernel to main OS package set to avoid duplicate kernels
	mergedSets[osPkgsKey] = mergedSets[osPkgsKey].Append(rpmmd.PackageSet{Include: []string{kernel}})
	return distro.MakePackageSetChains(t, mergedSets, repos)

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
	return t.packageSetChains
}

func (t *imageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	return []string{"assembler"}
}

// getBootType returns the BootType which should be used for this particular
// combination of architecture and image type.
func (t *imageType) getBootType() distro.BootType {
	bootType := t.arch.bootType
	if t.bootType != distro.UnsetBootType {
		bootType = t.bootType
	}
	return bootType
}

func (t *imageType) supportsUEFI() bool {
	bootType := t.getBootType()
	return bootType == distro.HybridBootType || bootType == distro.UEFIBootType
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

func (t *imageType) Manifest(customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecSets map[string][]rpmmd.PackageSpec,
	seed int64) (distro.Manifest, error) {

	if err := t.checkOptions(customizations, options); err != nil {
		return distro.Manifest{}, err
	}

	source := rand.NewSource(seed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	pipelines, err := t.pipelines(t, customizations, options, repos, packageSpecSets, rng)
	if err != nil {
		return distro.Manifest{}, err
	}

	// flatten spec sets for sources
	allPackageSpecs := make([]rpmmd.PackageSpec, 0)
	for _, specs := range packageSpecSets {
		allPackageSpecs = append(allPackageSpecs, specs...)
	}

	// handle OSTree commit inputs
	var commits []ostree.CommitSource
	if options.OSTree.Parent != "" && options.OSTree.URL != "" {
		commits = []ostree.CommitSource{{Checksum: options.OSTree.Parent, URL: options.OSTree.URL}}
	}

	// handle inline sources
	inlineData := []string{}

	// FDO root certs, if any, are transmitted via an inline source
	if fdo := customizations.GetFDO(); fdo != nil && fdo.DiunPubKeyRootCerts != "" {
		inlineData = append(inlineData, fdo.DiunPubKeyRootCerts)
	}

	return json.Marshal(
		osbuild.Manifest{
			Version:   "2",
			Pipelines: pipelines,
			Sources:   osbuild2.GenSources(allPackageSpecs, commits, inlineData),
		},
	)
}

func isMountpointAllowed(mountpoint string) bool {
	for _, allowed := range mountpointAllowList {
		match, _ := path.Match(allowed, mountpoint)
		if match {
			return true
		}
		// ensure that only clean mountpoints
		// are valid
		if strings.Contains(mountpoint, "//") {
			return false
		}
		match = strings.HasPrefix(mountpoint, allowed+"/")
		if allowed != "/" && match {
			return true
		}
	}
	return false
}

// checkOptions checks the validity and compatibility of options and customizations for the image type.
func (t *imageType) checkOptions(customizations *blueprint.Customizations, options distro.ImageOptions) error {
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
		if !isMountpointAllowed(m.Mountpoint) {
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
func NewF34() distro.Distro {
	return newDistro(fedora34Distribution)
}
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

	s390x := architecture{
		distro:   &rd,
		name:     distro.S390xArchName,
		bootType: distro.LegacyBootType,
	}

	ociImgType := qcow2ImgType
	ociImgType.name = "oci"

	x86_64.addImageTypes(
		amiImgType,
		qcow2ImgType,
		openstackImgType,
		vhdImgType,
		vmdkImgType,
		ociImgType,
		iotOCIImgType,
		iotCommitImgType,
		iotInstallerImgType,
	)
	aarch64.addImageTypes(
		amiImgType,
		qcow2ImgType,
		openstackImgType,
		ociImgType,
		iotCommitImgType,
		iotOCIImgType,
		iotInstallerImgType,
	)

	s390x.addImageTypes()

	rd.addArches(x86_64, aarch64, s390x)
	return &rd
}
