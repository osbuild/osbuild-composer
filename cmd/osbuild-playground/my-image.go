package main

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
)

type MyImage struct {
	MyOption string `json:"my_option"`
}

func (img *MyImage) Name() string {
	return "my-image"
}

func init() {
	AddImageType(&MyImage{})
}

func (img *MyImage) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	// Let's create a simple raw image!

	// configure a build pipeline
	build := manifest.NewBuild(m, runner, repos)
	build.Checkpoint()

	var build_platform platform.Platform
	switch common.CurrentArch() {
	case "aarch64":
		build_platform = &platform.Aarch64{}
	default:
		build_platform = &platform.X86{
			BIOS: true,
		}
	}

	// TODO: add helper
	pt, err := disk.NewPartitionTable(&basePT, nil, 0, false, rng)
	if err != nil {
		panic(err)
	}

	// create a minimal bootable OS tree
	os := manifest.NewOS(m, build, build_platform, repos)
	os.PartitionTable = pt   // we need a partition table
	os.KernelName = "kernel" // use the default fedora kernel
	os.OSCustomizations.Language = "en_US.UTF-8"
	os.OSCustomizations.Hostname = "my-host"
	os.OSCustomizations.Timezone = "UTC"

	// create a raw image containing the OS tree created above
	raw := manifest.NewRawImage(m, build, os)
	artifact := raw.Export()

	return artifact, nil
}
