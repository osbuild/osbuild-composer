package main

import (
	"fmt"
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
)

func RunPlayground(img ImageType, d distro.Distro, arch distro.Arch, repos map[string][]rpmmd.RepoConfig, state_dir string) {

	solver := dnfjson.NewSolver(d.ModulePlatformID(), d.Releasever(), arch.Name(), path.Join(state_dir, "rpmmd"))
	solver.SetDNFJSONPath(findDnfJsonBin())

	// Set cache size to 3 GiB
	solver.SetMaxCacheSize(1 * 1024 * 1024 * 1024)

	manifest := manifest.New()

	// TODO: query distro for runner
	err := img.InstantiateManifest(&manifest, repos[arch.Name()], &runner.Fedora{Version: 36})
	if err != nil {
		panic("InstantiateManifest() failed: " + err.Error())
	}

	packageSpecs := make(map[string][]rpmmd.PackageSpec)
	for name, chain := range manifest.GetPackageSetChains() {
		packages, err := solver.Depsolve(chain)
		if err != nil {
			panic(fmt.Sprintf("failed to depsolve for pipeline %s: %s\n", name, err.Error()))
		}
		packageSpecs[name] = packages
	}

	if err := solver.CleanCache(); err != nil {
		// print to stderr but don't exit with error
		fmt.Fprintf(os.Stderr, "could not clean dnf cache: %s", err.Error())
	}

	bytes, err := manifest.Serialize(packageSpecs)
	if err != nil {
		panic("failed to serialize manifest: " + err.Error())
	}

	store := path.Join(state_dir, "osbuild-store")

	_, err = osbuild2.RunOSBuild(bytes, store, "./", img.GetExports(), []string{"build"}, false, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not run osbuild: %s", err.Error())
	}
}
