package test_distro

import (
	"errors"
	"fmt"
	"sort"

	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/policies"
	"github.com/osbuild/images/pkg/rpmmd"
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
	// The Test Distro name base. It can't be used to get a distro.Distro
	// instance from the DistroFactory(), because it does not include any
	// release version.
	TestDistroNameBase = "test-distro"

	// An ID string for a Test Distro instance with release version 1.
	TestDistro1Name = TestDistroNameBase + "-1"

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

func (d *TestDistro) Codename() string {
	return "" // not supported
}

func (d *TestDistro) Releasever() string {
	return d.releasever
}

func (d *TestDistro) OsVersion() string {
	return d.releasever
}

func (d *TestDistro) Product() string {
	return d.name
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

func (t *TestImageType) ISOLabel() (string, error) {
	return "", nil
}

func (t *TestImageType) Size(size uint64) uint64 {
	if size == 0 {
		size = 1073741824
	}
	return size
}

func (t *TestImageType) PartitionType() disk.PartitionTableType {
	return disk.PT_NONE
}

func (t *TestImageType) BootMode() platform.BootMode {
	return platform.BOOT_HYBRID
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

func (t *TestImageType) Exports() []string {
	return distro.ExportsFallback()
}

func (t *TestImageType) Manifest(b *blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig, seedp *int64) (*manifest.Manifest, []string, error) {
	var bpPkgs []string
	if b != nil {
		mountpoints := b.Customizations.GetFilesystems()

		err := blueprint.CheckMountpointsPolicy(mountpoints, policies.MountpointPolicies)
		if err != nil {
			return nil, nil, err
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
			// handle the parameter combo error like we do in distros
			if ostreeOptions.ParentRef != "" && ostreeOptions.URL == "" {
				// specifying parent ref also requires URL
				return nil, nil, ostree.NewParameterComboError("ostree parent ref specified, but no URL to retrieve it")
			}
			if ostreeOptions.ImageRef != "" { // override with ref from image options
				ostreeSource.Ref = ostreeOptions.ImageRef
			}
			if ostreeOptions.ParentRef != "" { // override with parent ref
				ostreeSource.Ref = ostreeOptions.ParentRef
			}
			// copy any other options that might be specified
			ostreeSource.URL = options.OSTree.URL
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

	return m, nil, nil
}

// newTestDistro returns a new instance of TestDistro with the
// given release version.
//
// It contains two architectures "test_arch" and "test_arch2".
// "test_arch" contains one image type "test_type".
// "test_arch2" contains two image types "test_type" and "test_type2".
func newTestDistro(releasever string) *TestDistro {
	td := TestDistro{
		name:             fmt.Sprintf("%s-%s", TestDistroNameBase, releasever),
		releasever:       releasever,
		modulePlatformID: fmt.Sprintf("platform:%s-%s", TestDistroNameBase, releasever),
		ostreeRef:        fmt.Sprintf("test/%s/x86_64/edge", releasever),
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

func DistroFactory(idStr string) distro.Distro {
	id, err := distro.ParseID(idStr)
	if err != nil {
		return nil
	}

	if id.Name != TestDistroNameBase {
		return nil
	}

	if id.MinorVersion != -1 {
		return nil
	}

	return newTestDistro(fmt.Sprint(id.MajorVersion))
}
