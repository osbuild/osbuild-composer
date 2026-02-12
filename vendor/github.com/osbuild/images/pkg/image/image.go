package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type ImageKind interface {
	Name() string
	InstantiateManifest(m *manifest.Manifest, repos []rpmmd.RepoConfig, runner runner.Runner, rng *rand.Rand) (*artifact.Artifact, error)
}

type Base struct {
	name         string
	platform     platform.Platform
	filename     string
	BuildOptions *manifest.BuildOptions
}

func (img Base) Name() string {
	return img.name
}

func NewBase(name string, platform platform.Platform, filename string) Base {
	return Base{
		name:     name,
		platform: platform,
		filename: filename,
	}
}

func GetCompressionPipeline(compression string, buildPipeline manifest.Build, inputPipeline manifest.FilePipeline) manifest.FilePipeline {
	switch compression {
	case "xz":
		return manifest.NewXZ(buildPipeline, inputPipeline)
	case "zstd":
		return manifest.NewZstd(buildPipeline, inputPipeline)
	case "gzip":
		return manifest.NewGzip(buildPipeline, inputPipeline)
	case "":
		return inputPipeline
	default:
		// panic on unknown strings
		panic(fmt.Sprintf("unsupported compression type %q", compression))
	}
}
