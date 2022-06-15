package rhel90beta

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
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const defaultName = "rhel-90-beta"
const osVersion = "9.0"
const releaseVersion = "9"
const modulePlatformID = "platform:el9"
const ostreeRef = "rhel/9/%s/edge"

const (
	// package set names

	// build package set name
	buildPkgsKey = "build"

	// Legacy bootable image package set name
	bootLegacyPkgsKey = "boot.legacy"

	// UEFI bootable image package set name
	bootUEFIPkgsKey = "boot.uefi"

	// main/common os image package set name
	osPkgsKey = "packages"

	// edge os image package set name
	edgePkgsKey = "edge"

	// edge build package set name
	edgeBuildPkgsKey = "build.edge"

	// container package set name
	containerPkgsKey = "container"

	// installer package set name
	installerPkgsKey = "installer"

	// blueprint package set name
	blueprintPkgsKey = "blueprint"
)

var mountpointAllowList = []string{
	"/", "/var", "/opt", "/srv", "/usr", "/app", "/data", "/home",
}

type distribution struct {
	name             string
	modulePlatformID string
	ostreeRef        string
	arches           map[string]distro.Arch
	packageSets      map[string]rpmmd.PackageSet
}

func (d *distribution) Name() string {
	return d.name
}

func (d *distribution) Releasever() string {
	return releaseVersion
}

func (d *distribution) ModulePlatformID() string {
	return d.modulePlatformID
}

func (d *distribution) OSTreeRef() string {
	return d.ostreeRef
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

type architecture struct {
	distro           *distribution
	name             string
	imageTypes       map[string]distro.ImageType
	imageTypeAliases map[string]string
	packageSets      map[string]rpmmd.PackageSet
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

type imageType struct {
	arch             *architecture
	name             string
	nameAliases      []string
	filename         string
	mimeType         string
	packageSets      map[string]rpmmd.PackageSet
	packageSetChains map[string][]string
	enabledServices  []string
	disabledServices []string
	defaultTarget    string
	kernelOptions    string
	defaultSize      uint64
	buildPipelines   []string
	payloadPipelines []string
	exports          []string
	pipelines        pipelinesFunc

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
	if t.rpmOstree {
		return fmt.Sprintf(ostreeRef, t.Arch().Name())
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

func (t *imageType) PackageSets(bp blueprint.Blueprint, repos []rpmmd.RepoConfig) map[string][]rpmmd.PackageSet {
	// merge package sets that appear in the image type with the package sets
	// of the same name from the distro and arch
	mergedSets := make(map[string]rpmmd.PackageSet)

	imageSets := t.packageSets
	archSets := t.arch.packageSets
	distroSets := t.arch.distro.packageSets
	for name := range imageSets {
		mergedSets[name] = imageSets[name].Append(archSets[name]).Append(distroSets[name])
	}

	if _, hasPackages := imageSets[osPkgsKey]; !hasPackages {
		// should this be possible??
		mergedSets[osPkgsKey] = rpmmd.PackageSet{}
	}

	// build is usually not defined on the image type
	// handle it explicitly when it's not
	if _, hasBuild := imageSets[buildPkgsKey]; !hasBuild {
		mergedSets[buildPkgsKey] = archSets[buildPkgsKey].Append(distroSets[buildPkgsKey])
	}

	// package sets from flags
	if t.bootable {
		var addLegacyBootPkg bool
		var addUEFIBootPkg bool

		switch bt := t.getBootType(); bt {
		case distro.LegacyBootType:
			addLegacyBootPkg = true
		case distro.UEFIBootType:
			addUEFIBootPkg = true
		case distro.HybridBootType:
			addLegacyBootPkg = true
			addUEFIBootPkg = true
		default:
			panic(fmt.Sprintf("unsupported boot type: %q", bt))
		}

		if addLegacyBootPkg {
			mergedSets[osPkgsKey] = mergedSets[osPkgsKey].Append(archSets[bootLegacyPkgsKey]).Append(distroSets[bootLegacyPkgsKey])
		}
		if addUEFIBootPkg {
			mergedSets[osPkgsKey] = mergedSets[osPkgsKey].Append(archSets[bootUEFIPkgsKey]).Append(distroSets[bootUEFIPkgsKey])
		}
	}
	if t.rpmOstree {
		// add ostree sets
		mergedSets[buildPkgsKey] = mergedSets[buildPkgsKey].Append(archSets[edgeBuildPkgsKey]).Append(distroSets[edgeBuildPkgsKey])
		mergedSets[osPkgsKey] = mergedSets[osPkgsKey].Append(archSets[edgePkgsKey]).Append(distroSets[edgePkgsKey])
	}

	// blueprint packages
	bpPackages := bp.GetPackages()
	timezone, _ := bp.Customizations.GetTimezoneSettings()
	if timezone != nil {
		bpPackages = append(bpPackages, "chrony")
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
	return t.exports
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

	return disk.NewPartitionTable(&basePartitionTable, mountpoints, imageSize, false, rng)
}

func (t *imageType) PartitionType() string {
	archName := t.arch.Name()
	basePartitionTable, exists := t.basePartitionTables[archName]
	if !exists {
		return ""
	}

	return basePartitionTable.Type
}

// local type for ostree commit metadata used to define commit sources
type ostreeCommit struct {
	Checksum string
	URL      string
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

	var commits []ostreeCommit
	if t.bootISO && options.OSTree.Parent != "" && options.OSTree.URL != "" {
		commits = []ostreeCommit{{Checksum: options.OSTree.Parent, URL: options.OSTree.URL}}
	}
	return json.Marshal(
		osbuild.Manifest{
			Version:   "2",
			Pipelines: pipelines,
			Sources:   t.sources(allPackageSpecs, commits),
		},
	)
}

func (t *imageType) sources(packages []rpmmd.PackageSpec, ostreeCommits []ostreeCommit) osbuild.Sources {
	sources := osbuild.Sources{}
	curl := &osbuild.CurlSource{
		Items: make(map[string]osbuild.CurlSourceItem),
	}
	for _, pkg := range packages {
		item := new(osbuild.CurlSourceOptions)
		item.URL = pkg.RemoteLocation
		if pkg.Secrets == "org.osbuild.rhsm" {
			item.Secrets = &osbuild.URLSecrets{
				Name: "org.osbuild.rhsm",
			}
		}
		curl.Items[pkg.Checksum] = item
	}
	if len(curl.Items) > 0 {
		sources["org.osbuild.curl"] = curl
	}

	ostree := &osbuild.OSTreeSource{
		Items: make(map[string]osbuild.OSTreeSourceItem),
	}
	for _, commit := range ostreeCommits {
		item := new(osbuild.OSTreeSourceItem)
		item.Remote.URL = commit.URL
		ostree.Items[commit.Checksum] = *item
	}
	if len(ostree.Items) > 0 {
		sources["org.osbuild.ostree"] = ostree
	}
	return sources
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
		if t.name == "edge-installer" {
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

// New creates a new distro object, defining the supported architectures and image types
func New() distro.Distro {
	return newDistro(defaultName, modulePlatformID, ostreeRef)
}

func NewHostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	return newDistro(name, modulePlatformID, ostreeRef)
}

func newDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	const GigaByte = 1024 * 1024 * 1024

	rd := &distribution{
		name:             name,
		modulePlatformID: modulePlatformID,
		ostreeRef:        ostreeRef,
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey:     distroBuildPackageSet(),
			edgeBuildPkgsKey: edgeBuildPackageSet(),
		},
	}

	// Architecture definitions
	x86_64 := architecture{
		name:   distro.X86_64ArchName,
		distro: rd,
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey:      x8664BuildPackageSet(),
			bootLegacyPkgsKey: x8664LegacyBootPackageSet(),
			bootUEFIPkgsKey:   x8664UEFIBootPackageSet(),
			edgePkgsKey:       x8664EdgeCommitPackageSet(),
		},
		legacy:   "i386-pc",
		bootType: distro.HybridBootType,
	}

	aarch64 := architecture{
		name:   distro.Aarch64ArchName,
		distro: rd,
		packageSets: map[string]rpmmd.PackageSet{
			bootUEFIPkgsKey: aarch64UEFIBootPackageSet(),
			edgePkgsKey:     aarch64EdgeCommitPackageSet(),
		},
		bootType: distro.UEFIBootType,
	}

	ppc64le := architecture{
		distro: rd,
		name:   distro.Ppc64leArchName,
		packageSets: map[string]rpmmd.PackageSet{
			bootLegacyPkgsKey: ppc64leLegacyBootPackageSet(),
			buildPkgsKey:      ppc64leBuildPackageSet(),
		},
		legacy:   "powerpc-ieee1275",
		bootType: distro.LegacyBootType,
	}
	s390x := architecture{
		distro: rd,
		name:   distro.S390xArchName,
		packageSets: map[string]rpmmd.PackageSet{
			bootLegacyPkgsKey: s390xLegacyBootPackageSet(),
		},
		bootType: distro.LegacyBootType,
	}

	// Shared Services
	edgeServices := []string{
		"NetworkManager.service", "firewalld.service", "sshd.service",
		"greenboot-grub2-set-counter", "greenboot-grub2-set-success", "greenboot-healthcheck",
		"greenboot-rpm-ostree-grub2-check-fallback", "greenboot-status", "greenboot-task-runner",
		"redboot-auto-reboot", "redboot-task-runner",
	}

	// Image Definitions
	edgeCommitImgType := imageType{
		name:        "edge-commit",
		nameAliases: []string{"rhel-edge-commit"},
		filename:    "commit.tar",
		mimeType:    "application/x-tar",
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey: edgeBuildPackageSet(),
			osPkgsKey:    edgeCommitPackageSet(),
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		enabledServices:  edgeServices,
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
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey:     edgeBuildPackageSet(),
			osPkgsKey:        edgeCommitPackageSet(),
			containerPkgsKey: {Include: []string{"httpd"}},
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		enabledServices:  edgeServices,
		rpmOstree:        true,
		bootISO:          false,
		pipelines:        edgeContainerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "container-tree", "container"},
		exports:          []string{"container"},
	}
	edgeInstallerImgType := imageType{
		name:        "edge-installer",
		nameAliases: []string{"rhel-edge-installer"},
		filename:    "installer.iso",
		mimeType:    "application/x-iso9660-image",
		packageSets: map[string]rpmmd.PackageSet{
			// TODO: non-arch-specific package set handling for installers
			// This image type requires build packages for installers and
			// ostree/edge.  For now we only have x86-64 installer build
			// package sets defined.  When we add installer build package sets
			// for other architectures, this will need to be moved to the
			// architecture and the merging will happen in the PackageSets()
			// method like the other sets.
			buildPkgsKey:     x8664InstallerBuildPackageSet().Append(edgeBuildPackageSet()),
			osPkgsKey:        edgeCommitPackageSet(),
			installerPkgsKey: edgeInstallerPackageSet(),
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		enabledServices:  edgeServices,
		rpmOstree:        true,
		bootISO:          true,
		pipelines:        edgeInstallerPipelines,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "bootiso-tree", "bootiso"},
		exports:          []string{"bootiso"},
	}

	qcow2ImgType := imageType{
		name:          "qcow2",
		filename:      "disk.qcow2",
		mimeType:      "application/x-qemu-disk",
		defaultTarget: "multi-user.target",
		kernelOptions: "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0 crashkernel=auto",
		packageSets: map[string]rpmmd.PackageSet{
			osPkgsKey: qcow2CommonPackageSet(),
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
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
		packageSets: map[string]rpmmd.PackageSet{
			osPkgsKey: vhdCommonPackageSet(),
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		enabledServices: []string{
			"sshd",
			"waagent",
		},
		defaultTarget:       "multi-user.target",
		kernelOptions:       "ro biosdevname=0 rootdelay=300 console=ttyS0 earlyprintk=ttyS0 net.ifnames=0",
		bootable:            true,
		defaultSize:         4 * GigaByte,
		pipelines:           vhdPipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "vpc"},
		exports:             []string{"vpc"},
		basePartitionTables: defaultBasePartitionTables,
	}

	vmdkImgType := imageType{
		name:     "vmdk",
		filename: "disk.vmdk",
		mimeType: "application/x-vmdk",
		packageSets: map[string]rpmmd.PackageSet{
			osPkgsKey: vmdkCommonPackageSet(),
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
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
		packageSets: map[string]rpmmd.PackageSet{
			osPkgsKey: openstackCommonPackageSet(),
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
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

	// EC2 services
	ec2EnabledServices := []string{
		"sshd",
		"NetworkManager",
		"nm-cloud-setup.service",
		"nm-cloud-setup.timer",
		"cloud-init",
		"cloud-init-local",
		"cloud-config",
		"cloud-final",
		"reboot.target",
	}

	amiImgTypeX86_64 := imageType{
		name:     "ami",
		filename: "image.raw",
		mimeType: "application/octet-stream",
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey: ec2BuildPackageSet(),
			osPkgsKey:    ec2CommonPackageSet(),
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultTarget:       "multi-user.target",
		enabledServices:     ec2EnabledServices,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto",
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         10 * GigaByte,
		pipelines:           ec2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image"},
		exports:             []string{"image"},
		basePartitionTables: ec2BasePartitionTables,
	}

	amiImgTypeAarch64 := imageType{
		name:     "ami",
		filename: "image.raw",
		mimeType: "application/octet-stream",
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey: ec2BuildPackageSet(),
			osPkgsKey:    ec2CommonPackageSet(),
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultTarget:       "multi-user.target",
		enabledServices:     ec2EnabledServices,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 iommu.strict=0 crashkernel=auto",
		bootable:            true,
		defaultSize:         10 * GigaByte,
		pipelines:           ec2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image"},
		exports:             []string{"image"},
		basePartitionTables: ec2BasePartitionTables,
	}

	ec2ImgTypeX86_64 := imageType{
		name:     "ec2",
		filename: "image.raw.xz",
		mimeType: "application/xz",
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey: ec2BuildPackageSet(),
			osPkgsKey:    rhelEc2PackageSet(),
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultTarget:       "multi-user.target",
		enabledServices:     ec2EnabledServices,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto",
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         10 * GigaByte,
		pipelines:           rhelEc2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: ec2BasePartitionTables,
	}

	ec2ImgTypeAarch64 := imageType{
		name:     "ec2",
		filename: "image.raw.xz",
		mimeType: "application/xz",
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey: ec2BuildPackageSet(),
			osPkgsKey:    rhelEc2PackageSet(),
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultTarget:       "multi-user.target",
		enabledServices:     ec2EnabledServices,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 iommu.strict=0 crashkernel=auto",
		bootable:            true,
		defaultSize:         10 * GigaByte,
		pipelines:           rhelEc2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: ec2BasePartitionTables,
	}

	ec2HaImgTypeX86_64 := imageType{
		name:     "ec2-ha",
		filename: "image.raw.xz",
		mimeType: "application/xz",
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey: ec2BuildPackageSet(),
			osPkgsKey:    rhelEc2HaPackageSet(),
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultTarget:       "multi-user.target",
		enabledServices:     ec2EnabledServices,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto",
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         10 * GigaByte,
		pipelines:           rhelEc2Pipelines,
		buildPipelines:      []string{"build"},
		payloadPipelines:    []string{"os", "image", "archive"},
		exports:             []string{"archive"},
		basePartitionTables: ec2BasePartitionTables,
	}

	ec2SapImgTypeX86_64 := imageType{
		name:     "ec2-sap",
		filename: "image.raw.xz",
		mimeType: "application/xz",
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey: ec2BuildPackageSet(),
			osPkgsKey:    rhelEc2SapPackageSet(),
		},
		packageSetChains: map[string][]string{
			osPkgsKey: {osPkgsKey, blueprintPkgsKey},
		},
		defaultTarget:       "multi-user.target",
		enabledServices:     ec2EnabledServices,
		kernelOptions:       "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto processor.max_cstate=1 intel_idle.max_cstate=1",
		bootable:            true,
		bootType:            distro.LegacyBootType,
		defaultSize:         10 * GigaByte,
		pipelines:           rhelEc2SapPipelines,
		exports:             []string{"archive"},
		basePartitionTables: ec2BasePartitionTables,
	}

	tarImgType := imageType{
		name:     "tar",
		filename: "root.tar.xz",
		mimeType: "application/x-tar",
		packageSets: map[string]rpmmd.PackageSet{
			osPkgsKey: {
				Include: []string{"policycoreutils", "selinux-policy-targeted"},
				Exclude: []string{"rng-tools"},
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
	imageInstallerImgTypeX86_64 := imageType{
		name:     "image-installer",
		filename: "installer.iso",
		mimeType: "application/x-iso9660-image",
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey:     x8664InstallerBuildPackageSet(),
			osPkgsKey:        bareMetalPackageSet(),
			installerPkgsKey: installerPackageSet(),
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

	ociImgtype := qcow2ImgType
	ociImgtype.name = "oci"

	x86_64.addImageTypes(qcow2ImgType, vhdImgType, vmdkImgType, openstackImgType, amiImgTypeX86_64, ec2ImgTypeX86_64, ec2HaImgTypeX86_64, ec2SapImgTypeX86_64, tarImgType, imageInstallerImgTypeX86_64, edgeCommitImgType, edgeInstallerImgType, edgeOCIImgType, ociImgtype)
	aarch64.addImageTypes(qcow2ImgType, openstackImgType, amiImgTypeAarch64, ec2ImgTypeAarch64, tarImgType, edgeCommitImgType, edgeOCIImgType)
	ppc64le.addImageTypes(qcow2ImgType, tarImgType)
	s390x.addImageTypes(qcow2ImgType, tarImgType)

	rd.addArches(x86_64, aarch64, ppc64le, s390x)
	return rd
}
