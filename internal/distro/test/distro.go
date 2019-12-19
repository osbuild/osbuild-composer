package test

import (
	"errors"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type TestDistro struct{}

const Name = "test"

func New() *TestDistro {
	return &TestDistro{}
}

func (d *TestDistro) Name() string {
	return Name
}

func (d *TestDistro) Repositories(arch string) []rpmmd.RepoConfig {
	return []rpmmd.RepoConfig{
		{
			Id:      "test",
			Name:    "Test",
			BaseURL: "http://example.com/test/os",
		},
	}
}

func (d *TestDistro) ListOutputFormats() []string {
	return []string{}
}

func (d *TestDistro) FilenameFromType(outputFormat string) (string, string, error) {
	return "", "", errors.New("invalid output format: " + outputFormat)
}

func (d *TestDistro) Pipeline(b *blueprint.Blueprint, additionalRepos []rpmmd.RepoConfig, checksums map[string]string, outputArch, outputFormat string) (*pipeline.Pipeline, error) {
	return nil, errors.New("invalid output format or arch: " + outputFormat + " @ " + outputArch)
}

func (d *TestDistro) Runner() string {
	return "org.osbuild.test"
}
