package test_distro

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const (
	// package set names

	// build package set name
	buildPkgsKey = "build"

	// main/common os image package set name
	osPkgsKey = "packages"

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

	TestImageTypeName  = "test_type"
	TestImageType2Name = "test_type2"

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
	return t.architecture.distribution.OSTreeRef()
}

func (t *TestImageType) Size(size uint64) uint64 {
	return 0
}

func (t *TestImageType) PartitionType() string {
	return ""
}

func (t *TestImageType) PackageSets(bp blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig) map[string][]rpmmd.PackageSet {
	return map[string][]rpmmd.PackageSet{
		buildPkgsKey: {{
			Include: []string{
				"dep-package1",
				"dep-package2",
				"dep-package3",
			},
			Repositories: repos,
		},
		},
		blueprintPkgsKey: {{
			Include:      bp.GetPackages(),
			Repositories: repos,
		}},
		osPkgsKey: {{
			Include: []string{
				"dep-package1",
				"dep-package2",
				"dep-package3",
			},
			Repositories: repos,
		}},
	}
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

func (t *TestImageType) Manifest(b *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSpecSets map[string][]rpmmd.PackageSpec, seed int64) (distro.Manifest, error) {
	mountpoints := b.GetFilesystems()

	invalidMountpoints := []string{}
	for _, m := range mountpoints {
		if m.Mountpoint != "/" {
			invalidMountpoints = append(invalidMountpoints, m.Mountpoint)
		}
	}

	if len(invalidMountpoints) > 0 {
		return nil, fmt.Errorf("The following custom mountpoints are not supported %+q", invalidMountpoints)
	}

	return json.Marshal(
		osbuild.Manifest{
			Sources:   osbuild.Sources{},
			Pipelines: []osbuild.Pipeline{},
		},
	)
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

	ta1.addImageTypes(it1)
	ta2.addImageTypes(it1, it2)
	ta3.addImageTypes(it3, it4, it5, it6, it7, it8, it9, it10)

	td.addArches(&ta1, &ta2, &ta3)

	return &td
}

// New returns new instance of TestDistro named "test-distro".
func New() *TestDistro {
	return newTestDistro(TestDistroName, TestDistroModulePlatformID, TestDistroReleasever)
}

// New2 returns new instance of TestDistro named "test-distro-2".
func New2() *TestDistro {
	return newTestDistro(TestDistro2Name, TestDistro2ModulePlatformID, TestDistro2Releasever)
}
