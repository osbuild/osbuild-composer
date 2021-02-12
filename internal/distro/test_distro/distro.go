package test_distro

import (
	"encoding/json"
	"errors"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild1"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type TestDistro struct{}
type TestArch struct{}
type TestImageType struct{}

const name = "test-distro"
const modulePlatformID = "platform:test"

func (d *TestDistro) ListArches() []string {
	return []string{"test_arch"}
}

func (a *TestArch) Distro() distro.Distro {
	return &TestDistro{}
}

func (t *TestImageType) Arch() distro.Arch {
	return &TestArch{}
}

func (d *TestDistro) GetArch(arch string) (distro.Arch, error) {
	if arch != "test_arch" {
		return nil, errors.New("invalid arch: " + arch)
	}
	return &TestArch{}, nil
}

func (a *TestArch) Name() string {
	return "test_arch"
}

func (a *TestArch) ListImageTypes() []string {
	return []string{"test_type"}
}

func (a *TestArch) GetImageType(imageType string) (distro.ImageType, error) {
	if imageType != "test_type" {
		return nil, errors.New("invalid image type: " + imageType)
	}
	return &TestImageType{}, nil
}

func (t *TestImageType) Name() string {
	return "test_type"
}

func (t *TestImageType) Filename() string {
	return "test.img"
}

func (t *TestImageType) MIMEType() string {
	return "application/x-test"
}

func (t *TestImageType) OSTreeRef() string {
	return ""
}

func (t *TestImageType) Size(size uint64) uint64 {
	return 0
}

func (t *TestImageType) Packages(bp blueprint.Blueprint) ([]string, []string) {
	return nil, nil
}

func (t *TestImageType) BuildPackages() []string {
	return nil
}

func (t *TestImageType) Manifest(b *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, seed int64) (distro.Manifest, error) {
	return json.Marshal(
		osbuild.Manifest{
			Sources:  osbuild.Sources{},
			Pipeline: osbuild.Pipeline{},
		},
	)
}

func New() *TestDistro {
	return &TestDistro{}
}

func (d *TestDistro) Name() string {
	return name
}

func (d *TestDistro) ModulePlatformID() string {
	return modulePlatformID
}

func (d *TestDistro) FilenameFromType(outputFormat string) (string, string, error) {
	if outputFormat == "test_format" {
		return "test.img", "application/x-test", nil
	}

	return "", "", errors.New("invalid output format: " + outputFormat)
}
