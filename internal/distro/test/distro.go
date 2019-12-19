package test

import (
	"errors"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type TestDistro struct{}

const Name = "test-distro"

func New() *TestDistro {
	return &TestDistro{}
}

func (d *TestDistro) Name() string {
	return Name
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
	} else {
		return "", "", errors.New("invalid output format: " + outputFormat)
	}
}

func (d *TestDistro) Pipeline(b *blueprint.Blueprint, additionalRepos []rpmmd.RepoConfig, checksums map[string]string, outputArch, outputFormat string) (*pipeline.Pipeline, error) {
	if outputFormat == "test_output" && outputArch == "test_arch" {
		return &pipeline.Pipeline{}, nil
	} else {
		return nil, errors.New("invalid output format or arch: " + outputFormat + " @ " + outputArch)
	}
}

func (d *TestDistro) Runner() string {
	return "org.osbuild.test"
}
