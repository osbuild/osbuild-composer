package rhel85

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const defaultName = "rhel-85"
const osVersion = "8.5"
const modulePlatformID = "platform:el8"
const ostreeRef = "rhel/8/%s/edge"

const (
	// package set names

	// build package set name
	buildPkgsKey = "build"

	// bootable image package set name
	bootPkgsKey = "boot"

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

	for _, a := range arches {
		d.arches[a.name] = &architecture{
			distro:     d,
			name:       a.name,
			imageTypes: a.imageTypes,
		}
	}
}

type architecture struct {
	distro      *distribution
	name        string
	imageTypes  map[string]distro.ImageType
	packageSets map[string]rpmmd.PackageSet
	legacy      string
	uefi        bool
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
		return nil, errors.New("invalid image type: " + name)
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
	}
}

func (a *architecture) Distro() distro.Distro {
	return a.distro
}

type pipelinesFunc func(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error)

type imageType struct {
	arch             *architecture
	name             string
	filename         string
	mimeType         string
	packageSets      map[string]rpmmd.PackageSet
	enabledServices  []string
	disabledServices []string
	defaultTarget    string
	kernelOptions    string
	defaultSize      uint64
	exports          []string
	pipelines        pipelinesFunc

	// bootISO: installable ISO
	bootISO bool
	// rpmOstree: edge/ostree
	rpmOstree bool
	// bootable image
	bootable bool
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
		return fmt.Sprintf(ostreeRef, t.arch.name)
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

func (t *imageType) PackageSets(bp blueprint.Blueprint) map[string]rpmmd.PackageSet {
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
		// add boot sets
		mergedSets[osPkgsKey] = mergedSets[osPkgsKey].Append(archSets[bootPkgsKey]).Append(distroSets[bootPkgsKey])
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
	return mergedSets

}

func (t *imageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	return []string{"assembler"}
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
		item := new(osbuild.URLWithSecrets)
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

// checkOptions checks the validity and compatibility of options and customizations for the image type.
func (t *imageType) checkOptions(customizations *blueprint.Customizations, options distro.ImageOptions) error {
	if t.bootISO && t.rpmOstree {
		if options.OSTree.Parent == "" {
			return fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.name)
		}
		if customizations != nil {
			return fmt.Errorf("boot ISO image type %q does not support blueprint customizations", t.name)
		}
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.rpmOstree {
		return fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
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
		name:   "x86_64",
		distro: rd,
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey: x8664BuildPackageSet(),
			bootPkgsKey:  x8664BootPackageSet(),
			edgePkgsKey:  x8664EdgeCommitPackageSet(),
		},
		legacy: "i386-pc",
		uefi:   true,
	}

	aarch64 := architecture{
		name:   "aarch64",
		distro: rd,
		packageSets: map[string]rpmmd.PackageSet{
			bootPkgsKey: aarch64BootPackageSet(),
			edgePkgsKey: aarch64EdgeCommitPackageSet(),
		},
		uefi: true,
	}

	ppc64le := architecture{
		distro: rd,
		name:   "ppc64le",
		packageSets: map[string]rpmmd.PackageSet{
			bootPkgsKey:  ppc64leBootPackageSet(),
			buildPkgsKey: ppc64leBuildPackageSet(),
		},
		legacy: "powerpc-ieee1275",
		uefi:   false,
	}
	s390x := architecture{
		distro: rd,
		name:   "s390x",
		packageSets: map[string]rpmmd.PackageSet{
			bootPkgsKey: s390xBootPackageSet(),
		},
		uefi: false,
	}

	// Shared Services
	edgeServices := []string{
		"NetworkManager.service", "firewalld.service", "sshd.service",
	}

	// Image Definitions
	edgeCommitImgType := imageType{
		name:     "edge-commit",
		filename: "commit.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey: edgeBuildPackageSet(),
			osPkgsKey:    edgeCommitPackageSet(),
		},
		enabledServices: edgeServices,
		rpmOstree:       true,
		pipelines:       edgeCommitPipelines,
		exports:         []string{"commit-archive"},
	}
	edgeOCIImgType := imageType{
		name:     "edge-container",
		filename: "container.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey:     edgeBuildPackageSet(),
			osPkgsKey:        edgeCommitPackageSet(),
			containerPkgsKey: {Include: []string{"httpd"}},
		},
		enabledServices: edgeServices,
		rpmOstree:       true,
		bootISO:         false,
		pipelines:       edgeContainerPipelines,
		exports:         []string{containerPkgsKey},
	}
	edgeInstallerImgType := imageType{
		name:     "edge-installer",
		filename: "installer.iso",
		mimeType: "application/x-iso9660-image",
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey:     edgeBuildPackageSet(),
			osPkgsKey:        edgeCommitPackageSet(),
			installerPkgsKey: edgeInstallerPackageSet(),
		},
		enabledServices: edgeServices,
		rpmOstree:       true,
		bootISO:         true,
		pipelines:       edgeInstallerPipelines,
		exports:         []string{"bootiso"},
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
		bootable:    true,
		defaultSize: 10 * GigaByte,
		pipelines:   qcow2Pipelines,
		exports:     []string{"qcow2"},
	}

	vhdImgType := imageType{
		name:     "vhd",
		filename: "disk.vhd",
		mimeType: "application/x-vhd",
		packageSets: map[string]rpmmd.PackageSet{
			osPkgsKey: vhdCommonPackageSet(),
		},
		enabledServices: []string{
			"sshd",
			"waagent",
		},
		defaultTarget: "multi-user.target",
		kernelOptions: "ro biosdevname=0 rootdelay=300 console=ttyS0 earlyprintk=ttyS0 net.ifnames=0",
		bootable:      true,
		defaultSize:   4 * GigaByte,
		pipelines:     vhdPipelines,
		exports:       []string{"vhd"},
	}

	vmdkImgType := imageType{
		name:     "vmdk",
		filename: "disk.vmdk",
		mimeType: "application/x-vmdk",
		packageSets: map[string]rpmmd.PackageSet{
			osPkgsKey: vmdkCommonPackageSet(),
		},
		kernelOptions: "ro net.ifnames=0",
		bootable:      true,
		defaultSize:   4 * GigaByte,
		pipelines:     vmdkPipelines,
		exports:       []string{"vmdk"},
	}

	openstackImgType := imageType{
		name:     "openstack",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packageSets: map[string]rpmmd.PackageSet{
			osPkgsKey: openstackCommonPackageSet(),
		},
		kernelOptions: "ro net.ifnames=0",
		bootable:      true,
		defaultSize:   4 * GigaByte,
		pipelines:     openstackPipelines,
		exports:       []string{"qcow2"},
	}

	amiImgType := imageType{
		name:     "ami",
		filename: "image.raw",
		mimeType: "application/octet-stream",
		packageSets: map[string]rpmmd.PackageSet{
			osPkgsKey: amiCommonPackageSet(),
		},
		defaultTarget: "multi-user.target",
		kernelOptions: "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto",
		bootable:      true,
		defaultSize:   6 * GigaByte,
		pipelines:     amiPipelines,
		exports:       []string{"image"},
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
		pipelines: tarPipelines,
		exports:   []string{"root-tar"},
	}
	tarInstallerImgTypeX86_64 := imageType{
		name:     "tar-installer",
		filename: "installer.iso",
		mimeType: "application/x-iso9660-image",
		packageSets: map[string]rpmmd.PackageSet{
			buildPkgsKey: x8664InstallerBuildPackageSet(),
			osPkgsKey: {
				Include: []string{"lvm2", "policycoreutils", "selinux-policy-targeted"},
				Exclude: []string{"rng-tools"},
			},
			installerPkgsKey: installerPackageSet(),
		},
		rpmOstree: false,
		bootISO:   true,
		pipelines: tarInstallerPipelines,
		exports:   []string{"bootiso"},
	}

	x86_64.addImageTypes(qcow2ImgType, vhdImgType, vmdkImgType, openstackImgType, amiImgType, tarImgType, tarInstallerImgTypeX86_64, edgeCommitImgType, edgeInstallerImgType, edgeOCIImgType)
	aarch64.addImageTypes(qcow2ImgType, openstackImgType, amiImgType, tarImgType, edgeCommitImgType, edgeOCIImgType)
	ppc64le.addImageTypes(qcow2ImgType, tarImgType)
	s390x.addImageTypes(qcow2ImgType, tarImgType)

	rd.addArches(x86_64, aarch64, ppc64le, s390x)
	return rd
}
