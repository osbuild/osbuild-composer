package rhel90

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/oscap"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const (
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
	runner             string
	arches             map[string]distro.Arch
	defaultImageConfig *distro.ImageConfig
}

// RHEL-based OS image configuration defaults
var defaultDistroImageConfig = &distro.ImageConfig{
	Timezone: "America/New_York",
	Locale:   "C.UTF-8",
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

// distribution objects without the arches > image types
var distroMap = map[string]distribution{
	"rhel-90": {
		name:               "rhel-90",
		product:            "Red Hat Enterprise Linux",
		osVersion:          "9.0",
		releaseVersion:     "9",
		modulePlatformID:   "platform:el9",
		vendor:             "redhat",
		ostreeRefTmpl:      "rhel/9/%s/edge",
		isolabelTmpl:       "RHEL-9-0-0-BaseOS-%s",
		runner:             "org.osbuild.rhel90",
		defaultImageConfig: defaultDistroImageConfig,
	},
	"rhel-91": {
		name:               "rhel-91",
		product:            "Red Hat Enterprise Linux",
		osVersion:          "9.1",
		releaseVersion:     "9",
		modulePlatformID:   "platform:el9",
		vendor:             "redhat",
		ostreeRefTmpl:      "rhel/9/%s/edge",
		isolabelTmpl:       "RHEL-9-1-0-BaseOS-%s",
		runner:             "org.osbuild.rhel91",
		defaultImageConfig: defaultDistroImageConfig,
	},
	"centos-9": {
		name:               "centos-9",
		product:            "CentOS Stream",
		osVersion:          "9-stream",
		releaseVersion:     "9",
		modulePlatformID:   "platform:el9",
		vendor:             "centos",
		ostreeRefTmpl:      "centos/9/%s/edge",
		isolabelTmpl:       "CentOS-Stream-9-BaseOS-%s",
		runner:             "org.osbuild.centos9",
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

func (d *distribution) isRHEL() bool {
	return strings.HasPrefix(d.name, "rhel")
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

type pipelinesFunc func(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, containers []container.Spec, rng *rand.Rand) ([]osbuild.Pipeline, error)

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
	// rpmOstree: edge/ostree
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
		return fmt.Sprintf(d.ostreeRefTmpl, t.Arch().Name())
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

func (t *imageType) PackageSets(bp blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig) map[string][]rpmmd.PackageSet {
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
		archName := t.arch.Name()
		pt := t.basePartitionTables[archName]
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

	// if we are embedding containers we need to have `skopeo` in the build root
	if len(bp.Containers) > 0 {

		extraPkgs := rpmmd.PackageSet{Include: []string{"skopeo"}}

		if t.rpmOstree {
			// for OSTree based images we need to configure the containers-storage.conf(5)
			// via the org.osbuild.containers.storage.conf stage, which needs python3-toml
			extraPkgs = extraPkgs.Append(rpmmd.PackageSet{Include: []string{"python3-toml"}})
		}

		mergedSets[buildPkgsKey] = mergedSets[buildPkgsKey].Append(extraPkgs)
	}

	// if oscap customizations are enabled we need to add `openscap-scanner`
	// and `scap-security-guides` packages to build root
	if bp.Customizations.GetOpenSCAP() != nil {
		mergedSets[buildPkgsKey] = mergedSets[buildPkgsKey].Append(rpmmd.PackageSet{Include: []string{
			"openscap-scanner",
			"scap-security-guide",
		}})
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
	if bootType == distro.HybridBootType || bootType == distro.UEFIBootType {
		return true
	}
	return false
}

func (t *imageType) getPartitionTable(
	mountpoints []blueprint.FilesystemCustomization,
	options distro.ImageOptions,
	rng *rand.Rand,
) (*disk.PartitionTable, error) {
	archName := t.arch.Name()

	basePartitionTable, exists := t.basePartitionTables[archName]

	if !exists {
		return nil, fmt.Errorf("unknown arch: " + archName)
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
	archName := t.arch.Name()
	basePartitionTable, exists := t.basePartitionTables[archName]
	if !exists {
		return ""
	}

	return basePartitionTable.Type
}

func (t *imageType) Manifest(customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecSets map[string][]rpmmd.PackageSpec,
	containers []container.Spec,
	seed int64) (distro.Manifest, error) {

	if err := t.checkOptions(customizations, options, containers); err != nil {
		return distro.Manifest{}, err
	}

	source := rand.NewSource(seed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	pipelines, err := t.pipelines(t, customizations, options, repos, packageSpecSets, containers, rng)
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
			Sources:   osbuild.GenSources(allPackageSpecs, commits, inlineData, containers),
		},
	)
}

// checkOptions checks the validity and compatibility of options and customizations for the image type.
func (t *imageType) checkOptions(customizations *blueprint.Customizations, options distro.ImageOptions, containers []container.Spec) error {

	// we do not support embedding containers on ostree-derived images, only on commits themselves
	if len(containers) > 0 && t.rpmOstree && (t.name != "edge-commit" && t.name != "edge-container") {
		return fmt.Errorf("embedding containers is not supported for %s on %s", t.name, t.arch.distro.name)
	}

	if t.bootISO && t.rpmOstree {
		if options.OSTree.Parent == "" {
			return fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.name)
		}

		if t.name == "edge-simplified-installer" {
			allowed := []string{"InstallationDevice", "FDO"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
			}
			if customizations.GetInstallationDevice() == "" {
				return fmt.Errorf("boot ISO image type %q requires specifying an installation device to install to", t.name)
			}
			if customizations.GetFDO() == nil {
				return fmt.Errorf("boot ISO image type %q requires specifying FDO configuration to install to", t.name)
			}
			if customizations.GetFDO().ManufacturingServerURL == "" {
				return fmt.Errorf("boot ISO image type %q requires specifying FDO.ManufacturingServerURL configuration to install to", t.name)
			}
			var diunSet int
			if customizations.GetFDO().DiunPubKeyHash != "" {
				diunSet++
			}
			if customizations.GetFDO().DiunPubKeyInsecure != "" {
				diunSet++
			}
			if customizations.GetFDO().DiunPubKeyRootCerts != "" {
				diunSet++
			}
			if diunSet != 1 {
				return fmt.Errorf("boot ISO image type %q requires specifying one of [FDO.DiunPubKeyHash,FDO.DiunPubKeyInsecure,FDO.DiunPubKeyRootCerts] configuration to install to", t.name)
			}
		} else if t.name == "edge-installer" {
			allowed := []string{"User", "Group"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
			}
		}
	}

	if t.name == "edge-raw-image" && options.OSTree.Parent == "" {
		return fmt.Errorf("edge raw images require specifying a URL from which to retrieve the OSTree commit")
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.rpmOstree {
		return fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()

	if mountpoints != nil && t.rpmOstree {
		return fmt.Errorf("Custom mountpoints are not supported for ostree types")
	}

	err := disk.CheckMountpoints(mountpoints, disk.MountpointPolicies)
	if err != nil {
		return err
	}

	if osc := customizations.GetOpenSCAP(); osc != nil {
		if t.arch.distro.osVersion == "9.0" {
			return fmt.Errorf(fmt.Sprintf("OpenSCAP unsupported os version: %s", t.arch.distro.osVersion))
		}
		if !oscap.IsProfileAllowed(osc.ProfileID, oscapProfileAllowList) {
			return fmt.Errorf(fmt.Sprintf("OpenSCAP unsupported profile: %s", osc.ProfileID))
		}
		if t.rpmOstree {
			return fmt.Errorf("OpenSCAP customizations are not supported for ostree types")
		}
		if osc.DataStream == "" {
			return fmt.Errorf("OpenSCAP datastream cannot be empty")
		}
		if osc.ProfileID == "" {
			return fmt.Errorf("OpenSCAP profile cannot be empty")
		}
	}

	return nil
}

// New creates a new distro object, defining the supported architectures and image types
func New() distro.Distro {
	return newDistro("rhel-90")
}

func NewHostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	return newDistro("rhel-90")
}

func NewRHEL91() distro.Distro {
	return newDistro("rhel-91")
}

func NewRHEL91HostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	return newDistro("rhel-91")
}

func NewCentos() distro.Distro {
	return newDistro("centos-9")
}

func NewCentosHostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	return newDistro("centos-9")
}

func newDistro(distroName string) distro.Distro {
	const GigaByte = 1024 * 1024 * 1024

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

	// Shared Services
	edgeServices := []string{
		// TODO(runcom): move fdo-client-linuxapp.service to presets?
		"NetworkManager.service", "firewalld.service", "sshd.service", "fdo-client-linuxapp.service",
	}

	// Image Definitions
	edgeCommitImgType := imageType{
		name:        "edge-commit",
		nameAliases: []string{"rhel-edge-commit"},
		filename:    "commit.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: edgeBuildPackageSet,
			osPkgsKey:    edgeCommitPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: edgeServices,
		},
		rpmOstree:        true,
		pipelines:        edgeCommitPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "commit-archive"},
		exports:          []string{"commit-archive"},
	}

	edgeOCIImgType := imageType{
		name:        "edge-container",
		nameAliases: []string{"rhel-edge-container"},
		filename:    "container.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: edgeBuildPackageSet,
			osPkgsKey:    edgeCommitPackageSet,
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
			EnabledServices: edgeServices,
		},
		rpmOstree:        true,
		bootISO:          false,
		pipelines:        edgeContainerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "container-tree", "container"},
		exports:          []string{containerPkgsKey},
	}

	edgeRawImgType := imageType{
		name:        "edge-raw-image",
		nameAliases: []string{"rhel-edge-raw-image"},
		filename:    "image.raw.xz",
		mimeType:    "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: edgeRawImageBuildPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale: "en_US.UTF-8",
		},
		defaultSize:         10 * GigaByte,
		rpmOstree:           true,
		bootable:            true,
		bootISO:             false,
		pipelines:           edgeRawImagePipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"image-tree", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: edgeBasePartitionTables,
	}

	edgeInstallerImgType := imageType{
		name:        "edge-installer",
		nameAliases: []string{"rhel-edge-installer"},
		filename:    "installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			// TODO: non-arch-specific package set handling for installers
			// This image type requires build packages for installers and
			// ostree/edge.  For now we only have x86-64 installer build
			// package sets defined.  When we add installer build package sets
			// for other architectures, this will need to be moved to the
			// architecture and the merging will happen in the PackageSets()
			// method like the other sets.
			buildPkgsKey:     edgeInstallerBuildPackageSet,
			osPkgsKey:        edgeCommitPackageSet,
			installerPkgsKey: edgeInstallerPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			Locale:          "en_US.UTF-8",
			EnabledServices: edgeServices,
		},
		rpmOstree:        true,
		bootISO:          true,
		pipelines:        edgeInstallerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}

	edgeSimplifiedInstallerImgType := imageType{
		name:        "edge-simplified-installer",
		nameAliases: []string{"rhel-edge-simplified-installer"},
		filename:    "simplified-installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			// TODO: non-arch-specific package set handling for installers
			// This image type requires build packages for installers and
			// ostree/edge.  For now we only have x86-64 installer build
			// package sets defined.  When we add installer build package sets
			// for other architectures, this will need to be moved to the
			// architecture and the merging will happen in the PackageSets()
			// method like the other sets.
			buildPkgsKey:     edgeSimplifiedInstallerBuildPackageSet,
			installerPkgsKey: edgeSimplifiedInstallerPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			EnabledServices: edgeServices,
		},
		defaultSize:         10 * GigaByte,
		rpmOstree:           true,
		bootable:            true,
		bootISO:             true,
		pipelines:           edgeSimplifiedInstallerPipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"image-tree", "image", "archive", "coi-tree", "efiboot-tree", "bootiso-tree", "bootiso"},
		exports:             []string{"bootiso"},
		basePartitionTables: edgeBasePartitionTables,
	}

	qcow2ImgType := imageType{
		name:          "qcow2",
		filename:      "disk.qcow2",
		mimeType:      "application/x-qemu-disk",
		kernelOptions: "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    qcow2CommonPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig: &distro.ImageConfig{
			DefaultTarget: "multi-user.target",
			RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
				distro.RHSMConfigNoSubscription: {
					DnfPlugins: &osbuild.RHSMStageOptionsDnfPlugins{
						ProductID: &osbuild.RHSMStageOptionsDnfPlugin{
							Enabled: false,
						},
						SubscriptionManager: &osbuild.RHSMStageOptionsDnfPlugin{
							Enabled: false,
						},
					},
				},
			},
		},
		bootable:            true,
		defaultSize:         10 * GigaByte,
		pipelines:           qcow2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}

	vhdImgType := imageType{
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
		},
		kernelOptions:       "ro biosdevname=0 rootdelay=300 console=ttyS0 earlyprintk=ttyS0 net.ifnames=0",
		bootable:            true,
		defaultSize:         4 * GigaByte,
		pipelines:           vhdPipelines(false),
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vpc"},
		exports:             []string{"vpc"},
		basePartitionTables: defaultBasePartitionTables,
	}

	azureRhuiImgType := imageType{
		name:     "azure-rhui",
		filename: "disk.vhd.xz",
		mimeType: "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: ec2BuildPackageSet,
			osPkgsKey:    azureRhuiCommonPackageSet,
		},
		defaultImageConfig: &distro.ImageConfig{
			Timezone: "Etc/UTC",
			Locale:   "en_US.UTF-8",
			GPGKeyFiles: []string{
				"/etc/pki/rpm-gpg/RPM-GPG-KEY-microsoft-azure-release",
				"/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release",
			},
			Keyboard: &osbuild.KeymapStageOptions{
				Keymap: "us",
				X11Keymap: &osbuild.X11KeymapOptions{
					Layouts: []string{"us"},
				},
			},
			Sysconfig: []*osbuild.SysconfigStageOptions{
				{
					Kernel: &osbuild.SysconfigKernelOptions{
						UpdateDefault: true,
						DefaultKernel: "kernel-core",
					},
					Network: &osbuild.SysconfigNetworkOptions{
						Networking: true,
						NoZeroConf: true,
					},
				},
			},
			EnabledServices: []string{
				"firewalld",
				"nm-cloud-setup.service",
				"nm-cloud-setup.timer",
				"sshd",
				"waagent",
			},
			SshdConfig: &osbuild.SshdConfigStageOptions{
				Config: osbuild.SshdConfigConfig{
					ClientAliveInterval: common.IntToPtr(180),
				},
			},
			Modprobe: []*osbuild.ModprobeStageOptions{
				{
					Filename: "blacklist-amdgpu.conf",
					Commands: osbuild.ModprobeConfigCmdList{
						osbuild.NewModprobeConfigCmdBlacklist("amdgpu"),
					},
				},
				{
					Filename: "blacklist-floppy.conf",
					Commands: osbuild.ModprobeConfigCmdList{
						osbuild.NewModprobeConfigCmdBlacklist("floppy"),
					},
				},
				{
					Filename: "blacklist-nouveau.conf",
					Commands: osbuild.ModprobeConfigCmdList{
						osbuild.NewModprobeConfigCmdBlacklist("nouveau"),
						osbuild.NewModprobeConfigCmdBlacklist("lbm-nouveau"),
					},
				},
			},
			CloudInit: []*osbuild.CloudInitStageOptions{
				{
					Filename: "10-azure-kvp.cfg",
					Config: osbuild.CloudInitConfigFile{
						Reporting: &osbuild.CloudInitConfigReporting{
							Logging: &osbuild.CloudInitConfigReportingHandlers{
								Type: "log",
							},
							Telemetry: &osbuild.CloudInitConfigReportingHandlers{
								Type: "hyperv",
							},
						},
					},
				},
				{
					Filename: "91-azure_datasource.cfg",
					Config: osbuild.CloudInitConfigFile{
						Datasource: &osbuild.CloudInitConfigDatasource{
							Azure: &osbuild.CloudInitConfigDatasourceAzure{
								ApplyNetworkConfig: false,
							},
						},
						DatasourceList: []string{
							"Azure",
						},
					},
				},
			},
			PwQuality: &osbuild.PwqualityConfStageOptions{
				Config: osbuild.PwqualityConfConfig{
					Minlen:   common.IntToPtr(6),
					Minclass: common.IntToPtr(3),
					Dcredit:  common.IntToPtr(0),
					Ucredit:  common.IntToPtr(0),
					Lcredit:  common.IntToPtr(0),
					Ocredit:  common.IntToPtr(0),
				},
			},
			WAAgentConfig: &osbuild.WAAgentConfStageOptions{
				Config: osbuild.WAAgentConfig{
					RDFormat:     common.BoolToPtr(false),
					RDEnableSwap: common.BoolToPtr(false),
				},
			},
			RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
				distro.RHSMConfigNoSubscription: {
					DnfPlugins: &osbuild.RHSMStageOptionsDnfPlugins{
						SubscriptionManager: &osbuild.RHSMStageOptionsDnfPlugin{
							Enabled: false,
						},
					},
					SubMan: &osbuild.RHSMStageOptionsSubMan{
						Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
							AutoRegistration: common.BoolToPtr(true),
						},
						Rhsm: &osbuild.SubManConfigRHSMSection{
							ManageRepos: common.BoolToPtr(false),
						},
					},
				},
				distro.RHSMConfigWithSubscription: {
					SubMan: &osbuild.RHSMStageOptionsSubMan{
						Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
							AutoRegistration: common.BoolToPtr(true),
						},
						// do not disable the redhat.repo management if the user
						// explicitly request the system to be subscribed
					},
				},
			},
			Grub2Config: &osbuild.GRUB2Config{
				TerminalInput:  []string{"serial", "console"},
				TerminalOutput: []string{"serial", "console"},
				Serial:         "serial --speed=115200 --unit=0 --word=8 --parity=no --stop=1",
				Timeout:        10,
			},
			UdevRules: &osbuild.UdevRulesStageOptions{
				Filename: "/etc/udev/rules.d/68-azure-sriov-nm-unmanaged.rules",
				Rules: osbuild.UdevRules{
					osbuild.UdevRuleComment{
						Comment: []string{
							"Accelerated Networking on Azure exposes a new SRIOV interface to the VM.",
							"This interface is transparently bonded to the synthetic interface,",
							"so NetworkManager should just ignore any SRIOV interfaces.",
						},
					},
					osbuild.NewUdevRule(
						[]osbuild.UdevKV{
							{K: "SUBSYSTEM", O: "==", V: "net"},
							{K: "DRIVERS", O: "==", V: "hv_pci"},
							{K: "ACTION", O: "==", V: "add"},
							{K: "ENV", A: "NM_UNMANAGED", O: "=", V: "1"},
						},
					),
				},
			},
			SystemdUnit: []*osbuild.SystemdUnitStageOptions{
				{
					Unit:   "nm-cloud-setup.service",
					Dropin: "10-rh-enable-for-azure.conf",
					Config: osbuild.SystemdServiceUnitDropin{
						Service: &osbuild.SystemdUnitServiceSection{
							Environment: "NM_CLOUD_SETUP_AZURE=yes",
						},
					},
				},
			},
			DefaultTarget: "multi-user.target",
		},
		kernelOptions:       "ro console=tty1 console=ttyS0 earlyprintk=ttyS0 rootdelay=300",
		bootable:            true,
		defaultSize:         68719476736,
		pipelines:           vhdPipelines(true),
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vpc", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: azureRhuiBasePartitionTables,
	}

	vmdkImgType := imageType{
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
		},
		kernelOptions:       "ro net.ifnames=0",
		bootable:            true,
		defaultSize:         4 * GigaByte,
		pipelines:           vmdkPipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vmdk"},
		exports:             []string{"vmdk"},
		basePartitionTables: defaultBasePartitionTables,
	}

	openstackImgType := imageType{
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
		},
		kernelOptions:       "ro net.ifnames=0",
		bootable:            true,
		defaultSize:         4 * GigaByte,
		pipelines:           openstackPipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "qcow2"},
		exports:             []string{"qcow2"},
		basePartitionTables: defaultBasePartitionTables,
	}

	// default EC2 images config (common for all architectures)
	defaultEc2ImageConfig := &distro.ImageConfig{
		Locale:   "en_US.UTF-8",
		Timezone: "UTC",
		TimeSynchronization: &osbuild.ChronyStageOptions{
			Servers: []osbuild.ChronyConfigServer{
				{
					Hostname: "169.254.169.123",
					Prefer:   common.BoolToPtr(true),
					Iburst:   common.BoolToPtr(true),
					Minpoll:  common.IntToPtr(4),
					Maxpoll:  common.IntToPtr(4),
				},
			},
			// empty string will remove any occurrences of the option from the configuration
			LeapsecTz: common.StringToPtr(""),
		},
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
			X11Keymap: &osbuild.X11KeymapOptions{
				Layouts: []string{"us"},
			},
		},
		EnabledServices: []string{
			"sshd",
			"NetworkManager",
			"nm-cloud-setup.service",
			"nm-cloud-setup.timer",
			"cloud-init",
			"cloud-init-local",
			"cloud-config",
			"cloud-final",
			"reboot.target",
			"tuned",
		},
		DefaultTarget: "multi-user.target",
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
				NetworkScripts: &osbuild.NetworkScriptsOptions{
					IfcfgFiles: map[string]osbuild.IfcfgFile{
						"eth0": {
							Device:    "eth0",
							Bootproto: osbuild.IfcfgBootprotoDHCP,
							OnBoot:    common.BoolToPtr(true),
							Type:      osbuild.IfcfgTypeEthernet,
							UserCtl:   common.BoolToPtr(true),
							PeerDNS:   common.BoolToPtr(true),
							IPv6Init:  common.BoolToPtr(false),
						},
					},
				},
			},
		},
		RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
			distro.RHSMConfigNoSubscription: {
				// RHBZ#1932802
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					Rhsm: &osbuild.SubManConfigRHSMSection{
						ManageRepos: common.BoolToPtr(false),
					},
				},
			},
			distro.RHSMConfigWithSubscription: {
				// RHBZ#1932802
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					// do not disable the redhat.repo management if the user
					// explicitly request the system to be subscribed
				},
			},
		},
		SystemdLogind: []*osbuild.SystemdLogindStageOptions{
			{
				Filename: "00-getty-fixes.conf",
				Config: osbuild.SystemdLogindConfigDropin{

					Login: osbuild.SystemdLogindConfigLoginSection{
						NAutoVTs: common.IntToPtr(0),
					},
				},
			},
		},
		CloudInit: []*osbuild.CloudInitStageOptions{
			{
				Filename: "00-rhel-default-user.cfg",
				Config: osbuild.CloudInitConfigFile{
					SystemInfo: &osbuild.CloudInitConfigSystemInfo{
						DefaultUser: &osbuild.CloudInitConfigDefaultUser{
							Name: "ec2-user",
						},
					},
				},
			},
		},
		Modprobe: []*osbuild.ModprobeStageOptions{
			{
				Filename: "blacklist-nouveau.conf",
				Commands: osbuild.ModprobeConfigCmdList{
					osbuild.NewModprobeConfigCmdBlacklist("nouveau"),
				},
			},
		},
		DracutConf: []*osbuild.DracutConfStageOptions{
			{
				Filename: "sgdisk.conf",
				Config: osbuild.DracutConfigFile{
					Install: []string{"sgdisk"},
				},
			},
		},
		SystemdUnit: []*osbuild.SystemdUnitStageOptions{
			// RHBZ#1822863
			{
				Unit:   "nm-cloud-setup.service",
				Dropin: "10-rh-enable-for-ec2.conf",
				Config: osbuild.SystemdServiceUnitDropin{
					Service: &osbuild.SystemdUnitServiceSection{
						Environment: "NM_CLOUD_SETUP_EC2=yes",
					},
				},
			},
		},
		Authselect: &osbuild.AuthselectStageOptions{
			Profile: "sssd",
		},
		SshdConfig: &osbuild.SshdConfigStageOptions{
			Config: osbuild.SshdConfigConfig{
				PasswordAuthentication: common.BoolToPtr(false),
			},
		},
	}

	// default EC2 images config (x86_64)
	defaultEc2ImageConfigX86_64 := &distro.ImageConfig{
		DracutConf: append(defaultEc2ImageConfig.DracutConf,
			&osbuild.DracutConfStageOptions{
				Filename: "ec2.conf",
				Config: osbuild.DracutConfigFile{
					AddDrivers: []string{
						"nvme",
						"xen-blkfront",
					},
				},
			}),
	}
	defaultEc2ImageConfigX86_64 = defaultEc2ImageConfigX86_64.InheritFrom(defaultEc2ImageConfig)

	// default AMI (EC2 BYOS) images config
	defaultAMIImageConfig := &distro.ImageConfig{
		RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
			distro.RHSMConfigNoSubscription: {
				// RHBZ#1932802
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					// Don't disable RHSM redhat.repo management on the AMI
					// image, which is BYOS and does not use RHUI for content.
					// Otherwise subscribing the system manually after booting
					// it would result in empty redhat.repo. Without RHUI, such
					// system would have no way to get Red Hat content, but
					// enable the repo management manually, which would be very
					// confusing.
				},
			},
			distro.RHSMConfigWithSubscription: {
				// RHBZ#1932802
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					// do not disable the redhat.repo management if the user
					// explicitly request the system to be subscribed
				},
			},
		},
	}
	defaultAMIImageConfigX86_64 := defaultAMIImageConfig.InheritFrom(defaultEc2ImageConfigX86_64)
	defaultAMIImageConfig = defaultAMIImageConfig.InheritFrom(defaultEc2ImageConfig)

	amiImgTypeX86_64 := imageType{
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
		defaultImageConfig:  defaultAMIImageConfigX86_64,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295",
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         10 * GigaByte,
		pipelines:           ec2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image"},
		exports:             []string{"image"},
		basePartitionTables: defaultBasePartitionTables,
	}

	amiImgTypeAarch64 := imageType{
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
		defaultImageConfig:  defaultAMIImageConfig,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 iommu.strict=0",
		bootable:            true,
		defaultSize:         10 * GigaByte,
		pipelines:           ec2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image"},
		exports:             []string{"image"},
		basePartitionTables: defaultBasePartitionTables,
	}

	ec2ImgTypeX86_64 := imageType{
		name:     "ec2",
		filename: "image.raw.xz",
		mimeType: "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: ec2BuildPackageSet,
			osPkgsKey:    rhelEc2PackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultEc2ImageConfigX86_64,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295",
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         10 * GigaByte,
		pipelines:           rhelEc2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	ec2ImgTypeAarch64 := imageType{
		name:     "ec2",
		filename: "image.raw.xz",
		mimeType: "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: ec2BuildPackageSet,
			osPkgsKey:    rhelEc2PackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultEc2ImageConfig,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 iommu.strict=0",
		bootable:            true,
		defaultSize:         10 * GigaByte,
		pipelines:           rhelEc2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	ec2HaImgTypeX86_64 := imageType{
		name:     "ec2-ha",
		filename: "image.raw.xz",
		mimeType: "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: ec2BuildPackageSet,
			osPkgsKey:    rhelEc2HaPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultEc2ImageConfigX86_64,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295",
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         10 * GigaByte,
		pipelines:           rhelEc2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	// default EC2-SAP image config (x86_64)
	defaultEc2SapImageConfigX86_64 := &distro.ImageConfig{
		SELinuxConfig: &osbuild.SELinuxConfigStageOptions{
			State: osbuild.SELinuxStatePermissive,
		},
		// RHBZ#1960617
		Tuned: osbuild.NewTunedStageOptions("sap-hana"),
		// RHBZ#1959979
		Tmpfilesd: []*osbuild.TmpfilesdStageOptions{
			osbuild.NewTmpfilesdStageOptions("sap.conf",
				[]osbuild.TmpfilesdConfigLine{
					{
						Type: "x",
						Path: "/tmp/.sap*",
					},
					{
						Type: "x",
						Path: "/tmp/.hdb*lock",
					},
					{
						Type: "x",
						Path: "/tmp/.trex*lock",
					},
				},
			),
		},
		// RHBZ#1959963
		PamLimitsConf: []*osbuild.PamLimitsConfStageOptions{
			osbuild.NewPamLimitsConfStageOptions("99-sap.conf",
				[]osbuild.PamLimitsConfigLine{
					{
						Domain: "@sapsys",
						Type:   osbuild.PamLimitsTypeHard,
						Item:   osbuild.PamLimitsItemNofile,
						Value:  osbuild.PamLimitsValueInt(1048576),
					},
					{
						Domain: "@sapsys",
						Type:   osbuild.PamLimitsTypeSoft,
						Item:   osbuild.PamLimitsItemNofile,
						Value:  osbuild.PamLimitsValueInt(1048576),
					},
					{
						Domain: "@dba",
						Type:   osbuild.PamLimitsTypeHard,
						Item:   osbuild.PamLimitsItemNofile,
						Value:  osbuild.PamLimitsValueInt(1048576),
					},
					{
						Domain: "@dba",
						Type:   osbuild.PamLimitsTypeSoft,
						Item:   osbuild.PamLimitsItemNofile,
						Value:  osbuild.PamLimitsValueInt(1048576),
					},
					{
						Domain: "@sapsys",
						Type:   osbuild.PamLimitsTypeHard,
						Item:   osbuild.PamLimitsItemNproc,
						Value:  osbuild.PamLimitsValueUnlimited,
					},
					{
						Domain: "@sapsys",
						Type:   osbuild.PamLimitsTypeSoft,
						Item:   osbuild.PamLimitsItemNproc,
						Value:  osbuild.PamLimitsValueUnlimited,
					},
					{
						Domain: "@dba",
						Type:   osbuild.PamLimitsTypeHard,
						Item:   osbuild.PamLimitsItemNproc,
						Value:  osbuild.PamLimitsValueUnlimited,
					},
					{
						Domain: "@dba",
						Type:   osbuild.PamLimitsTypeSoft,
						Item:   osbuild.PamLimitsItemNproc,
						Value:  osbuild.PamLimitsValueUnlimited,
					},
				},
			),
		},
		// RHBZ#1959962
		Sysctld: []*osbuild.SysctldStageOptions{
			osbuild.NewSysctldStageOptions("sap.conf",
				[]osbuild.SysctldConfigLine{
					{
						Key:   "kernel.pid_max",
						Value: "4194304",
					},
					{
						Key:   "vm.max_map_count",
						Value: "2147483647",
					},
				},
			),
		},
		// E4S/EUS
		DNFConfig: []*osbuild.DNFConfigStageOptions{
			osbuild.NewDNFConfigStageOptions(
				[]osbuild.DNFVariable{
					{
						Name:  "releasever",
						Value: rd.osVersion,
					},
				},
				nil,
			),
		},
	}
	defaultEc2SapImageConfigX86_64 = defaultEc2SapImageConfigX86_64.InheritFrom(defaultEc2ImageConfigX86_64)

	ec2SapImgTypeX86_64 := imageType{
		name:     "ec2-sap",
		filename: "image.raw.xz",
		mimeType: "application/xz",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: ec2BuildPackageSet,
			osPkgsKey:    rhelEc2SapPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultEc2SapImageConfigX86_64,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 processor.max_cstate=1 intel_idle.max_cstate=1",
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         10 * GigaByte,
		pipelines:           rhelEc2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	defaultGceImageConfig := &distro.ImageConfig{
		Timezone: "UTC",
		TimeSynchronization: &osbuild.ChronyStageOptions{
			Timeservers: []string{"metadata.google.internal"},
		},
		Firewall: &osbuild.FirewallStageOptions{
			DefaultZone: "trusted",
		},
		EnabledServices: []string{
			"sshd",
			"rngd",
			"dnf-automatic.timer",
		},
		DisabledServices: []string{
			"sshd-keygen@",
			"reboot.target",
		},
		DefaultTarget: "multi-user.target",
		Locale:        "en_US.UTF-8",
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
		},
		DNFConfig: []*osbuild.DNFConfigStageOptions{
			{
				Config: &osbuild.DNFConfig{
					Main: &osbuild.DNFConfigMain{
						IPResolve: "4",
					},
				},
			},
		},
		DNFAutomaticConfig: &osbuild.DNFAutomaticConfigStageOptions{
			Config: &osbuild.DNFAutomaticConfig{
				Commands: &osbuild.DNFAutomaticConfigCommands{
					ApplyUpdates: common.BoolToPtr(true),
					UpgradeType:  osbuild.DNFAutomaticUpgradeTypeSecurity,
				},
			},
		},
		YUMRepos: []*osbuild.YumReposStageOptions{
			{
				Filename: "google-cloud.repo",
				Repos: []osbuild.YumRepository{
					{
						Id:           "google-compute-engine",
						Name:         "Google Compute Engine",
						BaseURL:      []string{"https://packages.cloud.google.com/yum/repos/google-compute-engine-el9-x86_64-stable"},
						Enabled:      common.BoolToPtr(true),
						GPGCheck:     common.BoolToPtr(true),
						RepoGPGCheck: common.BoolToPtr(false),
						GPGKey: []string{
							"https://packages.cloud.google.com/yum/doc/yum-key.gpg",
							"https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg",
						},
					},
					{
						Id:           "google-cloud-sdk",
						Name:         "Google Cloud SDK",
						BaseURL:      []string{"https://packages.cloud.google.com/yum/repos/cloud-sdk-el9-x86_64"},
						Enabled:      common.BoolToPtr(true),
						GPGCheck:     common.BoolToPtr(true),
						RepoGPGCheck: common.BoolToPtr(false),
						GPGKey: []string{
							"https://packages.cloud.google.com/yum/doc/yum-key.gpg",
							"https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg",
						},
					},
				},
			},
		},
		RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
			distro.RHSMConfigNoSubscription: {
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					// Don't disable RHSM redhat.repo management on the GCE
					// image, which is BYOS and does not use RHUI for content.
					// Otherwise subscribing the system manually after booting
					// it would result in empty redhat.repo. Without RHUI, such
					// system would have no way to get Red Hat content, but
					// enable the repo management manually, which would be very
					// confusing.
				},
			},
			distro.RHSMConfigWithSubscription: {
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					// do not disable the redhat.repo management if the user
					// explicitly request the system to be subscribed
				},
			},
		},
		SshdConfig: &osbuild.SshdConfigStageOptions{
			Config: osbuild.SshdConfigConfig{
				PasswordAuthentication: common.BoolToPtr(false),
				ClientAliveInterval:    common.IntToPtr(420),
				PermitRootLogin:        osbuild.PermitRootLoginValueNo,
			},
		},
		Sysconfig: []*osbuild.SysconfigStageOptions{
			{
				Kernel: &osbuild.SysconfigKernelOptions{
					DefaultKernel: "kernel-core",
					UpdateDefault: true,
				},
			},
		},
		Modprobe: []*osbuild.ModprobeStageOptions{
			{
				Filename: "blacklist-floppy.conf",
				Commands: osbuild.ModprobeConfigCmdList{
					osbuild.NewModprobeConfigCmdBlacklist("floppy"),
				},
			},
		},
	}

	gceImgType := imageType{
		name:     "gce",
		filename: "image.tar.gz",
		mimeType: "application/gzip",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    gcePackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultGceImageConfig,
		kernelOptions:       "net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y console=ttyS0,38400n8d",
		bootable:            true,
		bootType:            distro.UEFIBootType,
		defaultSize:         20 * GigaByte,
		pipelines:           gcePipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	defaultGceRhuiImageConfig := &distro.ImageConfig{
		RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
			distro.RHSMConfigNoSubscription: {
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					Rhsm: &osbuild.SubManConfigRHSMSection{
						ManageRepos: common.BoolToPtr(false),
					},
				},
			},
			distro.RHSMConfigWithSubscription: {
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					// do not disable the redhat.repo management if the user
					// explicitly request the system to be subscribed
				},
			},
		},
	}
	defaultGceRhuiImageConfig = defaultGceRhuiImageConfig.InheritFrom(defaultGceImageConfig)

	gceRhuiImgType := imageType{
		name:     "gce-rhui",
		filename: "image.tar.gz",
		mimeType: "application/gzip",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    gceRhuiPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultImageConfig:  defaultGceRhuiImageConfig,
		kernelOptions:       "net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y console=ttyS0,38400n8d",
		bootable:            true,
		bootType:            distro.UEFIBootType,
		defaultSize:         20 * GigaByte,
		pipelines:           gcePipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: defaultBasePartitionTables,
	}

	tarImgType := imageType{
		name:     "tar",
		filename: "root.tar.xz",
		mimeType: "application/x-tar",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey: func(t *imageType) rpmmd.PackageSet {
				return rpmmd.PackageSet{
					Include: []string{"policycoreutils", "selinux-policy-targeted"},
					Exclude: []string{"rng-tools"},
				}
			},
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		pipelines:        tarPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "root-tar"},
		exports:          []string{"root-tar"},
	}
	imageInstaller := imageType{
		name:     "image-installer",
		filename: "installer.iso",
		mimeType: "application/x-iso9660-image",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey:     anacondaBuildPackageSet,
			osPkgsKey:        bareMetalPackageSet,
			installerPkgsKey: anacondaPackageSet,
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		rpmOstree:        false,
		bootISO:          true,
		bootable:         true,
		pipelines:        imageInstallerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"os", "anaconda-tree", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}

	ociImgType := qcow2ImgType
	ociImgType.name = "oci"

	x86_64.addImageTypes(qcow2ImgType, vhdImgType, vmdkImgType, openstackImgType, amiImgTypeX86_64, tarImgType, imageInstaller, edgeCommitImgType, edgeInstallerImgType, edgeOCIImgType, edgeRawImgType, edgeSimplifiedInstallerImgType, ociImgType, gceImgType)
	aarch64.addImageTypes(qcow2ImgType, openstackImgType, amiImgTypeAarch64, tarImgType, imageInstaller, edgeCommitImgType, edgeInstallerImgType, edgeOCIImgType, edgeRawImgType, edgeSimplifiedInstallerImgType)
	ppc64le.addImageTypes(qcow2ImgType, tarImgType)
	s390x.addImageTypes(qcow2ImgType, tarImgType)

	if rd.isRHEL() {
		// add azure to RHEL distro only
		x86_64.addImageTypes(azureRhuiImgType)

		// add ec2 image types to RHEL distro only
		x86_64.addImageTypes(ec2ImgTypeX86_64, ec2HaImgTypeX86_64, ec2SapImgTypeX86_64)
		aarch64.addImageTypes(ec2ImgTypeAarch64)

		// add GCE RHUI image to RHEL only
		x86_64.addImageTypes(gceRhuiImgType)
	}
	rd.addArches(x86_64, aarch64, ppc64le, s390x)
	return &rd
}
