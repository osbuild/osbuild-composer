package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora"
	"github.com/osbuild/osbuild-composer/internal/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const (
	store  = "/media/scratch/osbuild-store"
	source = "/home/achilleas/projects/osbuild/osbuild-composer"
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

func makeSolver() distro.PackageResolver {
	solver := dnfjson.NewSolver("platform:f37", "37", "x86_64", "fedora-37", path.Join(store, "rpmmd"))
	solver.SetDNFJSONPath(filepath.Join(source, "./dnf-json"))

	// Set cache size to 3 GiB
	solver.SetMaxCacheSize(1 * 1024 * 1024 * 1024)

	return func(chains map[string][]rpmmd.PackageSet) (map[string][]rpmmd.PackageSpec, error) {
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
}

func build(it distro.ImageType) {
	bp := new(blueprint.Blueprint)
	bp.Name = "playground"

	check(bp.Initialize())

	options := distro.ImageOptions{
		Size: 0,
	}

	repos := getRepos(it.Arch().Distro().Name(), it.Arch().Name())

	contSolver := func(source, name string, tlsVerify *bool) (container.Spec, error) {
		r := container.NewResolver(it.Arch().Name())
		r.Add(source, name, tlsVerify)
		res, err := r.Finish()
		if err != nil {
			return container.Spec{}, err
		}
		return res[0], nil
	}

	m, _, err := it.Manifest(bp, options, repos, 0, makeSolver(), contSolver)
	check(err)

	write_manifest(m)

	// outputDir := "./"
	// extraEnv := []string{}
	// jsonResult := false
	// _, err = osbuild.RunOSBuild(bytes, store, outputDir, m.GetExports(), m.GetCheckpoints(), extraEnv, jsonResult, os.Stdout)
	// check(err)

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
