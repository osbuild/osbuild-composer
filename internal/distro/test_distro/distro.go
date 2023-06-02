package test_distro

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	dnfjson_mock "github.com/osbuild/osbuild-composer/internal/mocks/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const (
	// package set names

	// build package set name
	buildPkgsKey = "build"

	// main/common os image package set name
	osPkgsKey = "os"

	// blueprint package set name
	blueprintPkgsKey = "blueprint"
)

type TestDistro struct {
	name             string
	releasever       string
	modulePlatformID string
	ostreeRef        string
	arches           map[string]distro.Arch
}

type TestArch struct {
	distribution *TestDistro
	name         string
	imageTypes   map[string]distro.ImageType
}

type TestImageType struct {
	architecture *TestArch
	name         string
}

const (
	TestDistroName              = "test-distro"
	TestDistro2Name             = "test-distro-2"
	TestDistroReleasever        = "1"
	TestDistro2Releasever       = "2"
	TestDistroModulePlatformID  = "platform:test"
	TestDistro2ModulePlatformID = "platform:test-2"

	TestArchName  = "test_arch"
	TestArch2Name = "test_arch2"
	TestArch3Name = "test_arch3"

	TestImageTypeName   = "test_type"
	TestImageType2Name  = "test_type2"
	TestImageTypeOSTree = "test_ostree_type"

	// added for cloudapi tests
	TestImageTypeAmi            = "ami"
	TestImageTypeGce            = "gce"
	TestImageTypeVhd            = "vhd"
	TestImageTypeEdgeCommit     = "rhel-edge-commit"
	TestImageTypeEdgeInstaller  = "rhel-edge-installer"
	TestImageTypeImageInstaller = "image-installer"
	TestImageTypeQcow2          = "qcow2"
	TestImageTypeVmdk           = "vmdk"
)

// TestDistro

func (d *TestDistro) Name() string {
	return d.name
}

func (d *TestDistro) Releasever() string {
	return d.releasever
}

func (d *TestDistro) ModulePlatformID() string {
	return d.modulePlatformID
}

func (d *TestDistro) OSTreeRef() string {
	return d.ostreeRef
}

func (d *TestDistro) ListArches() []string {
	archs := make([]string, 0, len(d.arches))
	for name := range d.arches {
		archs = append(archs, name)
	}
	sort.Strings(archs)
	return archs
}

func (d *TestDistro) GetArch(arch string) (distro.Arch, error) {
	a, exists := d.arches[arch]
	if !exists {
		return nil, errors.New("invalid arch: " + arch)
	}
	return a, nil
}

func (d *TestDistro) addArches(arches ...*TestArch) {
	if d.arches == nil {
		d.arches = map[string]distro.Arch{}
	}

	for _, a := range arches {
		a.distribution = d
		d.arches[a.Name()] = a
	}
}

// TestArch

func (a *TestArch) Name() string {
	return a.name
}

func (a *TestArch) Distro() distro.Distro {
	return a.distribution
}

func (a *TestArch) ListImageTypes() []string {
	formats := make([]string, 0, len(a.imageTypes))
	for name := range a.imageTypes {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (a *TestArch) GetImageType(imageType string) (distro.ImageType, error) {
	t, exists := a.imageTypes[imageType]
	if !exists {
		return nil, errors.New("invalid image type: " + imageType)
	}

	return t, nil
}

func (a *TestArch) addImageTypes(imageTypes ...TestImageType) {
	if a.imageTypes == nil {
		a.imageTypes = map[string]distro.ImageType{}
	}
	for idx := range imageTypes {
		it := imageTypes[idx]
		it.architecture = a
		a.imageTypes[it.Name()] = &it
	}
}

// TestImageType

func (t *TestImageType) Name() string {
	return t.name
}

func (t *TestImageType) Arch() distro.Arch {
	return t.architecture
}

func (t *TestImageType) Filename() string {
	return "test.img"
}

func (t *TestImageType) MIMEType() string {
	return "application/x-test"
}

func (t *TestImageType) OSTreeRef() string {
	if t.name == TestImageTypeEdgeCommit || t.name == TestImageTypeEdgeInstaller || t.name == TestImageTypeOSTree {
		return t.architecture.distribution.OSTreeRef()
	}
	return ""
}

func (t *TestImageType) Size(size uint64) uint64 {
	return 0
}

func (t *TestImageType) PartitionType() string {
	return ""
}

func (t *TestImageType) BootMode() distro.BootMode {
	return distro.BOOT_HYBRID
}

func (t *TestImageType) BuildPipelines() []string {
	return distro.BuildPipelinesFallback()
}

func (t *TestImageType) PayloadPipelines() []string {
	return distro.PayloadPipelinesFallback()
}

func (t *TestImageType) PayloadPackageSets() []string {
	return []string{blueprintPkgsKey}
}

func (t *TestImageType) PackageSetsChains() map[string][]string {
	return map[string][]string{
		osPkgsKey: {osPkgsKey, blueprintPkgsKey},
	}
}

func (t *TestImageType) Exports() []string {
	return distro.ExportsFallback()
}

func (t *TestImageType) Manifest(b *blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig, seed int64) (*manifest.Manifest, []string, error) {
	var bpPkgs []string
	if b != nil {
		mountpoints := b.Customizations.GetFilesystems()

		invalidMountpoints := []string{}
		for _, m := range mountpoints {
			if m.Mountpoint != "/" {
				invalidMountpoints = append(invalidMountpoints, m.Mountpoint)
			}
		}

		if len(invalidMountpoints) > 0 {
			return nil, nil, fmt.Errorf("The following custom mountpoints are not supported %+q", invalidMountpoints)
		}

		bpPkgs = b.GetPackages()
	}

	var ostreeSources []ostree.SourceSpec
	if defaultRef := t.OSTreeRef(); defaultRef != "" {
		// ostree image type
		ostreeSource := ostree.SourceSpec{ // init with default
			Ref: defaultRef,
		}
		if ostreeOptions := options.OSTree; ostreeOptions != nil {
			if ostreeOptions.ImageRef != "" { // override with ref from image options
				ostreeSource.Ref = ostreeOptions.ImageRef
			}
			// copy any other options that might be specified
			ostreeSource.URL = options.OSTree.URL
			ostreeSource.Parent = options.OSTree.ParentRef
			ostreeSource.RHSM = options.OSTree.RHSM
		}
		ostreeSources = []ostree.SourceSpec{ostreeSource}
	}

	buildPackages := []rpmmd.PackageSet{{
		Include: []string{
			"dep-package1",
			"dep-package2",
			"dep-package3",
		},
		Repositories: repos,
	}}
	osPackages := []rpmmd.PackageSet{
		{
			Include:      bpPkgs,
			Repositories: repos,
		},
		{
			Include: []string{
				"dep-package1",
				"dep-package2",
				"dep-package3",
			},
			Repositories: repos,
		},
	}

	m := &manifest.Manifest{}

	manifest.NewContentTest(m, buildPkgsKey, buildPackages, nil, nil)
	manifest.NewContentTest(m, osPkgsKey, osPackages, nil, ostreeSources)

	m.Content.PackageSets = m.GetPackageSetChains()
	m.Content.Containers = m.GetContainerSourceSpecs()
	m.Content.OSTreeCommits = m.GetOSTreeSourceSpecs()

	return m, nil, nil
}

// newTestDistro returns a new instance of TestDistro with the
// given name and modulePlatformID.
//
// It contains two architectures "test_arch" and "test_arch2".
// "test_arch" contains one image type "test_type".
// "test_arch2" contains two image types "test_type" and "test_type2".
func newTestDistro(name, modulePlatformID, releasever string) *TestDistro {
	td := TestDistro{
		name:             name,
		releasever:       releasever,
		modulePlatformID: modulePlatformID,
		ostreeRef:        "test/13/x86_64/edge",
	}

	ta1 := TestArch{
		name: TestArchName,
	}

	ta2 := TestArch{
		name: TestArch2Name,
	}

	ta3 := TestArch{
		name: TestArch3Name,
	}

	it1 := TestImageType{
		name: TestImageTypeName,
	}

	it2 := TestImageType{
		name: TestImageType2Name,
	}

	it3 := TestImageType{
		name: TestImageTypeAmi,
	}

	it4 := TestImageType{
		name: TestImageTypeVhd,
	}

	it5 := TestImageType{
		name: TestImageTypeEdgeCommit,
	}

	it6 := TestImageType{
		name: TestImageTypeEdgeInstaller,
	}

	it7 := TestImageType{
		name: TestImageTypeImageInstaller,
	}

	it8 := TestImageType{
		name: TestImageTypeQcow2,
	}

	it9 := TestImageType{
		name: TestImageTypeVmdk,
	}

	it10 := TestImageType{
		name: TestImageTypeGce,
	}

	it11 := TestImageType{
		name: TestImageTypeOSTree,
	}

	ta1.addImageTypes(it1, it11)
	ta2.addImageTypes(it1, it2)
	ta3.addImageTypes(it3, it4, it5, it6, it7, it8, it9, it10)

	td.addArches(&ta1, &ta2, &ta3)

	return &td
}

// New returns new instance of TestDistro named "test-distro".
func New() *TestDistro {
	return newTestDistro(TestDistroName, TestDistroModulePlatformID, TestDistroReleasever)
}

func NewRegistry() *distroregistry.Registry {
	td := New()
	registry, err := distroregistry.New(td, td)
	if err != nil {
		panic(err)
	}

	// Override the host's architecture name with the test's name
	registry.SetHostArchName(TestArchName)
	return registry
}

// New2 returns new instance of TestDistro named "test-distro-2".
func New2() *TestDistro {
	return newTestDistro(TestDistro2Name, TestDistro2ModulePlatformID, TestDistro2Releasever)
}

// ResolveContent transforms content source specs into resolved specs for serialization.
// For packages, it uses the dnfjson_mock.BaseDeps() every time, but retains
// the map keys from the input.
// For ostree commits it hashes the URL+Ref to create a checksum.
func ResolveContent(pkgs map[string][]rpmmd.PackageSet, containers map[string][]container.SourceSpec, commits map[string][]ostree.SourceSpec) (map[string][]rpmmd.PackageSpec, map[string][]container.Spec, map[string][]ostree.CommitSpec) {

	pkgSpecs := make(map[string][]rpmmd.PackageSpec, len(pkgs))
	for name := range pkgs {
		pkgSpecs[name] = dnfjson_mock.BaseDeps()
	}

	containerSpecs := make(map[string][]container.Spec, len(containers))
	for name := range containers {
		containerSpecs[name] = make([]container.Spec, len(containers[name]))
		for idx := range containers[name] {
			containerSpecs[name][idx] = container.Spec{
				Source:    containers[name][idx].Source,
				TLSVerify: containers[name][idx].TLSVerify,
				LocalName: containers[name][idx].Name,
			}
		}
	}

	commitSpecs := make(map[string][]ostree.CommitSpec, len(commits))
	for name := range commits {
		commitSpecs[name] = make([]ostree.CommitSpec, len(commits[name]))
		for idx := range commits[name] {
			commitSpecs[name][idx] = ostree.CommitSpec{
				Ref:      commits[name][idx].Ref,
				URL:      commits[name][idx].URL,
				Checksum: fmt.Sprintf("%x", sha256.Sum256([]byte(commits[name][idx].URL+commits[name][idx].Ref))),
			}
			fmt.Printf("Test distro spec: %+v\n", commitSpecs[name][idx])
		}
	}

	return pkgSpecs, containerSpecs, commitSpecs
}
