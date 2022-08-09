package image

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
)

type ImageKind interface {
	Name() string
	InstantiateManifest(m *manifest.Manifest, repos []rpmmd.RepoConfig, runner runner.Runner, rng *rand.Rand) (*artifact.Artifact, error)
}

type Base struct {
	name string
}

func (img Base) Name() string {
	return img.name
}

func NewBase(name string) Base {
	return Base{
		name: name,
	}
}
