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
	var imageTypeArg string
	var blueprintArg string
	var archArg string
	var distroArg string
	var rpmmdArg bool
	flag.StringVar(&imageTypeArg, "image-type", "", "image type, e.g. qcow2 or ami")
	flag.StringVar(&archArg, "arch", "", "architecture to create image for, e.g. x86_64")
	flag.StringVar(&distroArg, "distro", "", "distribution to create, e.g. fedora-30")
	flag.BoolVar(&rpmmdArg, "rpmmd", false, "output rpmmd struct instead of pipeline manifest")
	flag.Parse()

	// Path to blueprint or '-' for stdin
	blueprintArg = flag.Arg(0)

	// Print help usage if one of the required arguments wasn't provided
	if imageTypeArg == "" || archArg == "" || distroArg == "" {
		flag.Usage()
		return
	}

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

	d := distros.GetDistro(distroArg)
	if d == nil {
		_, _ = fmt.Fprintf(os.Stderr, "The provided distribution (%s) is not supported. Use one of these:\n", distroArg)
		for _, distro := range distros.List() {
			_, _ = fmt.Fprintln(os.Stderr, " *", distro)
		}
		return
	}

	arch, err := d.GetArch(archArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The provided architecture (%s) is not supported by distro %s.: %s\n", archArg, distroArg, err.Error())
		return
	}

	imageType, err := arch.GetImageType(imageTypeArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The provided image type (%s) is not supported by architecture %s on distro %s.: %s\n", imageTypeArg, archArg, distroArg, err.Error())
		return
	}

	repos, err := rpmmd.LoadRepositories([]string{"."}, distroArg)
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
	packageSpecs, checksums, err := rpmmd.Depsolve(packages, exclude_pkgs, repos[archArg], d.ModulePlatformID())
	if err != nil {
		panic("Could not depsolve: " + err.Error())
	}

	buildPkgs := imageType.BuildPackages()
	buildPackageSpecs, _, err := rpmmd.Depsolve(buildPkgs, nil, repos[archArg], d.ModulePlatformID())
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
			panic("could not marshal rpmmd struct into JSON")
		}
	} else {
		manifest, err := imageType.Manifest(blueprint.Customizations, repos[archArg], packageSpecs, buildPackageSpecs, imageType.Size(0))
		if err != nil {
			panic(err.Error())
		}

		bytes, err = json.Marshal(manifest)
		if err != nil {
			panic("could not marshal manifest into JSON")
		}
	}
	os.Stdout.Write(bytes)
}
