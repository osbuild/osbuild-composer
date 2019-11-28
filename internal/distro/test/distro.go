package test

import (
	"errors"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type TestDistro struct{}

func init() {
	distro.Register("test", &TestDistro{})
}

func (d *TestDistro) Repositories() []rpmmd.RepoConfig {
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

func (d *TestDistro) Pipeline(b *blueprint.Blueprint, outputFormat string) (*pipeline.Pipeline, error) {
	return nil, errors.New("invalid output format: " + outputFormat)
}

func (d *TestDistro) Runner() string {
	return "org.osbuild.test"
}
