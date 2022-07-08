package main

import (
	"math/rand"

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

func (img *MyImage) InstantiateManifest(m *manifest.Manifest, repos []rpmmd.RepoConfig, runner runner.Runner) error {
	// Let's create a simple raw image!

	// configure a build pipeline
	build := manifest.NewBuild(m, runner, repos)

	// create an x86_64 platform with bios boot
	platform := &platform.X86{
		BIOS: true,
	}

	// TODO: add helper
	// math/rand is good enough in this case
	/* #nosec G404 */
	pt, err := disk.NewPartitionTable(&basePT, nil, 0, false, rand.New(rand.NewSource(0)))
	if err != nil {
		panic(err)
	}

	// create a minimal bootable OS tree
	os := manifest.NewOS(m, build, platform, repos)
	os.PartitionTable = pt   // we need a partition table
	os.KernelName = "kernel" // use the default fedora kernel

	// create a raw image containing the OS tree created above
	manifest.NewRawImage(m, build, os)

	return nil
}

// TODO: make internal
func (img *MyImage) GetExports() []string {
	return []string{"image"}
}

func (img *MyImage) GetCheckpoints() []string {
	return []string{"build"}
}
