package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora"
	"github.com/osbuild/osbuild-composer/internal/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const (
	store  = "/tmp/osbuild-composer/"
	source = "."
)

func getRepos(distro, arch string) []rpmmd.RepoConfig {
	distroRepos, err := rpmmd.LoadRepositories([]string{filepath.Join(source, "test/data/")}, distro)
	check(err)
	return distroRepos[arch]
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func write_manifest(bytes []byte) {
	fname := "manifest.json"
	fp, err := os.Create(fname)
	check(err)
	_, err = fp.Write(bytes)
	check(err)
	fmt.Printf("Saved manifest to %s\n", fname)
}

func depsolve(chains map[string][]rpmmd.PackageSet) (map[string][]rpmmd.PackageSpec, error) {
	solver := dnfjson.NewSolver("platform:f37", "37", "x86_64", "fedora-37", path.Join(store, "rpmmd"))
	solver.SetDNFJSONPath(filepath.Join(source, "./dnf-json"))

	// Set cache size to 3 GiB
	solver.SetMaxCacheSize(1 * 1024 * 1024 * 1024)

	solved := make(map[string][]rpmmd.PackageSpec, len(chains))
	for name, pkgSet := range chains {
		res, err := solver.Depsolve(pkgSet)
		if err != nil {
			return nil, err
		}
		solved[name] = res
	}
	if err := solver.CleanCache(); err != nil {
		// log and ignore
		fmt.Printf("Error during rpm repo cache cleanup: %s", err.Error())
	}
	return solved, nil
}

func build(it distro.ImageType) {
	bp := new(blueprint.Blueprint)
	bp.Name = "playground"

	check(bp.Initialize())

	options := distro.ImageOptions{
		Size: 0,
	}

	repos := getRepos(it.Arch().Distro().Name(), it.Arch().Name())

	pkgSets := it.PackageSets(*bp, options, repos)
	pkgs, err := depsolve(pkgSets)
	check(err)

	m, _, err := it.Manifest(bp.Customizations, options, repos, pkgs, nil, 0)
	check(err)

	write_manifest(m)

	fmt.Println("Done")
}

func main() {
	distro := fedora.NewF37()
	arch, err := distro.GetArch("x86_64")
	check(err)

	it, err := arch.GetImageType("qcow2")
	check(err)

	build(it)
}
