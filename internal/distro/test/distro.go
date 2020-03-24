package test

import (
	"errors"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type TestDistro struct{}
type testArch struct{}
type testImageType struct{}

const name = "test-distro"
const modulePlatformID = "platform:test"

func (d *TestDistro) ListArchs() []string {
	return []string{"test_arch"}
}

func (d *TestDistro) GetArch(arch string) (distro.Arch, error) {
	if arch != "test_arch" {
		return nil, errors.New("invalid arch: " + arch)
	}
	return &testArch{}, nil
}

func (a *testArch) Name() string {
	return "test_format"
}

func (a *testArch) ListImageTypes() []string {
	return []string{"test-format"}
}

func (a *testArch) GetImageType(imageType string) (distro.ImageType, error) {
	if imageType != "test_output" {
		return nil, errors.New("invalid image type: " + imageType)
	}
	return &testImageType{}, nil
}

func (t *testImageType) Name() string {
	return "test-format"
}

func (t *testImageType) Filename() string {
	return "test.img"
}

func (t *testImageType) MIMEType() string {
	return "application/x-test"
}

func (t *testImageType) Size(size uint64) uint64 {
	return 0
}

func (t *testImageType) BasePackages() ([]string, []string) {
	return nil, nil
}

func (t *testImageType) BuildPackages() []string {
	return nil
}

func (t *testImageType) Manifest(b *blueprint.Customizations, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, size uint64) (*osbuild.Manifest, error) {
	return &osbuild.Manifest{
		Sources:  osbuild.Sources{},
		Pipeline: osbuild.Pipeline{},
	}, nil
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
