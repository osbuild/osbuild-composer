package test

import (
	"errors"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type TestDistro struct{}

const Name = "test-distro"
const ModulePlatformID = "platform:test"

func New() *TestDistro {
	return &TestDistro{}
}

func (d *TestDistro) Name() string {
	return Name
}

func (d *TestDistro) ModulePlatformID() string {
	return ModulePlatformID
}

func (d *TestDistro) Repositories(arch string) []rpmmd.RepoConfig {
	return []rpmmd.RepoConfig{
		{
			Id:      "test-id",
			Name:    "Test Name",
			BaseURL: "http://example.com/test/os/" + arch,
		},
	}
}

func (d *TestDistro) ListOutputFormats() []string {
	return []string{"test_format"}
}

func (d *TestDistro) FilenameFromType(outputFormat string) (string, string, error) {
	if outputFormat == "test_format" {
		return "test.img", "application/x-test", nil
	}

	return "", "", errors.New("invalid output format: " + outputFormat)
}

func (d *TestDistro) BasePackages(outputFormat, outputArchitecture string) ([]string, []string, error) {
	return nil, nil, nil
}

func (d *TestDistro) BuildPackages(outputArchitecture string) ([]string, error) {
	return nil, nil
}

func (d *TestDistro) Pipeline(b *blueprint.Blueprint, additionalRepos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, checksums map[string]string, outputArch, outputFormat string, size uint64) (*osbuild.Pipeline, error) {
	if outputFormat == "test_output" && outputArch == "test_arch" {
		return &osbuild.Pipeline{}, nil
	}

	return nil, errors.New("invalid output format or arch: " + outputFormat + " @ " + outputArch)
}

func (d *TestDistro) Sources(packages []rpmmd.PackageSpec) *osbuild.Sources {
	return &osbuild.Sources{}
}

func (r *TestDistro) Manifest(b *blueprint.Blueprint, additionalRepos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, checksums map[string]string, outputArchitecture, outputFormat string, size uint64) (*osbuild.Manifest, error) {
	pipeline, err := r.Pipeline(b, additionalRepos, packageSpecs, buildPackageSpecs, checksums, outputArchitecture, outputFormat, size)
	if err != nil {
		return nil, err
	}

	return &osbuild.Manifest{
		Sources:  *r.Sources(append(packageSpecs, buildPackageSpecs...)),
		Pipeline: *pipeline,
	}, nil
}

func (d *TestDistro) Runner() string {
	return "org.osbuild.test"
}
