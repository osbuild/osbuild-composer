package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/rpmmd"
	"github.com/osbuild/image-builder/pkg/runner"
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

func GetCompressionPipeline(compression string, buildPipeline manifest.Build, inputPipeline manifest.FilePipeline, name string) manifest.FilePipeline {
	switch compression {
	case "xz":
		return manifest.NewXZ(buildPipeline, inputPipeline, name)
	case "zstd":
		return manifest.NewZstd(buildPipeline, inputPipeline, name)
	case "gzip":
		return manifest.NewGzip(buildPipeline, inputPipeline, name)
	case "":
		return inputPipeline
	default:
		// panic on unknown strings
		panic(fmt.Sprintf("unsupported compression type %q", compression))
	}
}

func compressionExt(compression string) string {
	switch compression {
	case "xz":
		return "xz"
	case "zstd":
		return "zst"
	case "gzip":
		return "gz"
	default:
		panic(fmt.Sprintf("unsupported compression type %q", compression))
	}
}
