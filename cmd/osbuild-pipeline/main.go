package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/osbuild/osbuild-composer/internal/distro/fedora30"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel81"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel82"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type rpmMD struct {
	BuildPackages []rpmmd.PackageSpec `json:"build-packages"`
	Packages      []rpmmd.PackageSpec `json:"packages"`
	Checksums     map[string]string   `json:"checksums"`
}

func main() {
	var distroArg string
	var archArg string
	var imageTypeArg string
	var blueprintArg string
	var rpmmdArg bool
	flag.StringVar(&distroArg, "distro", "", "distribution to create, e.g. fedora-30")
	flag.StringVar(&archArg, "arch", "", "architecture to create image for, e.g. x86_64")
	flag.StringVar(&imageTypeArg, "image-type", "", "image type, e.g. qcow2 or ami")
	flag.BoolVar(&rpmmdArg, "rpmmd", false, "output rpmmd struct instead of pipeline manifest")
	flag.Parse()

	// Path to blueprint or '-' for stdin
	blueprintArg = flag.Arg(0)

	blueprint := &blueprint.Blueprint{}
	if blueprintArg != "" {
		var reader io.Reader
		if blueprintArg == "-" {
			reader = os.Stdin
		} else {
			var err error
			reader, err = os.Open(blueprintArg)
			if err != nil {
				panic("Could not open bluerpint: " + err.Error())
			}
		}
		file, err := ioutil.ReadAll(reader)
		if err != nil {
			panic("Could not read blueprint: " + err.Error())
		}
		err = json.Unmarshal(file, &blueprint)
		if err != nil {
			panic("Could not parse blueprint: " + err.Error())
		}
	}

	distros, err := distro.NewRegistry(fedora30.New(), fedora31.New(), fedora32.New(), rhel81.New(), rhel82.New())
	if err != nil {
		panic(err)
	}

	distro := distros.GetDistro(distroArg)
	if distro == nil {
		_, _ = fmt.Fprintf(os.Stderr, "The provided distribution '%s' is not supported. Use one of these:\n", distroArg)
		for _, d := range distros.List() {
			_, _ = fmt.Fprintln(os.Stderr, " *", d)
		}
		return
	}

	arch, err := distro.GetArch(archArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The provided architecture '%s' is not supported by %s. Use one of these:\n", archArg, distro.Name())
		for _, a := range distro.ListArchs() {
			_, _ = fmt.Fprintln(os.Stderr, " *", a)
		}
		return
	}

	imageType, err := arch.GetImageType(imageTypeArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The provided image type '%s' is not supported by %s for %s. Use one of these:\n", imageTypeArg, distro.Name(), arch.Name())
		for _, t := range arch.ListImageTypes() {
			_, _ = fmt.Fprintln(os.Stderr, " *", t)
		}
		return
	}

	repos, err := rpmmd.LoadRepositories([]string{"."}, distro.Name())
	if err != nil {
		panic(err)
	}

	packages := make([]string, len(blueprint.Packages))
	for i, pkg := range blueprint.Packages {
		packages[i] = pkg.Name
		// If a package has version "*" the package name suffix must be equal to "-*-*.*"
		// Using just "-*" would find any other package containing the package name
		if pkg.Version != "" && pkg.Version != "*" {
			packages[i] += "-" + pkg.Version
		} else if pkg.Version == "*" {
			packages[i] += "-*-*.*"
		}
	}

	pkgs, exclude_pkgs := imageType.BasePackages()
	packages = append(pkgs, packages...)

	home, err := os.UserHomeDir()
	if err != nil {
		panic("os.UserHomeDir(): " + err.Error())
	}

	rpmmd := rpmmd.NewRPMMD(path.Join(home, ".cache/osbuild-composer/rpmmd"))
	packageSpecs, checksums, err := rpmmd.Depsolve(packages, exclude_pkgs, repos[arch.Name()], distro.ModulePlatformID(), arch.Name())
	if err != nil {
		panic("Could not depsolve: " + err.Error())
	}

	buildPkgs := imageType.BuildPackages()
	buildPackageSpecs, _, err := rpmmd.Depsolve(buildPkgs, nil, repos[arch.Name()], distro.ModulePlatformID(), arch.Name())
	if err != nil {
		panic("Could not depsolve build packages: " + err.Error())
	}

	var bytes []byte
	if rpmmdArg {
		rpmMDInfo := rpmMD{
			BuildPackages: buildPackageSpecs,
			Packages:      packageSpecs,
			Checksums:     checksums,
		}
		bytes, err = json.Marshal(rpmMDInfo)
		if err != nil {
			panic(err)
		}
	} else {
		manifest, err := imageType.Manifest(blueprint.Customizations, repos[arch.Name()], packageSpecs, buildPackageSpecs, imageType.Size(0))
		if err != nil {
			panic(err.Error())
		}

		bytes, err = json.Marshal(manifest)
		if err != nil {
			panic(err)
		}
	}
	os.Stdout.Write(bytes)
}
