package main

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
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

	// create an x86_64 platform with bios boot
	platform := &platform.X86{
		BIOS: true,
	}

	// TODO: add helper
	pt, err := disk.NewPartitionTable(&basePT, nil, 0, false, rng)
	if err != nil {
		panic(err)
	}

	// create a minimal bootable OS tree
	os := manifest.NewOS(m, build, platform, repos)
	os.PartitionTable = pt   // we need a partition table
	os.KernelName = "kernel" // use the default fedora kernel

	// create a raw image containing the OS tree created above
	raw := manifest.NewRawImage(m, build, os)
	artifact := raw.Export()

	return artifact, nil
}
