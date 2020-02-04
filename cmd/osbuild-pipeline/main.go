package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/osbuild/osbuild-composer/internal/common"
	"io/ioutil"
	"os"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func main() {
	var imageType string
	var blueprintArg string
	var archArg string
	var distroArg string
	flag.StringVar(&imageType, "image-type", "", "image type, e.g. qcow2 or ami")
	flag.StringVar(&blueprintArg, "blueprint", "", "path to a JSON file containing a blueprint to translate")
	flag.StringVar(&archArg, "arch", "", "architecture to create image for, e.g. x86_64")
	flag.StringVar(&distroArg, "distro", "", "distribution to create, e.g. fedora-30")
	flag.Parse()

	// Print help usage if one of the required arguments wasn't provided
	if imageType == "" || blueprintArg == "" || archArg == "" || distroArg == "" {
		flag.Usage()
		return
	}

	// Validate architecture
	if !common.ArchitectureExists(archArg) {
		_, _ = fmt.Fprintf(os.Stderr, "The provided architecture (%s) is not supported. Use one of these:\n", archArg)
		for _, arch := range common.ListArchitectures() {
			_, _ = fmt.Fprintln(os.Stderr, " *", arch)
		}
		return
	}

	// Validate distribution
	if !common.DistributionExists(distroArg) {
		_, _ = fmt.Fprintf(os.Stderr, "The provided distribution (%s) is not supported. Use one of these:\n", distroArg)
		for _, distro := range common.ListDistributions() {
			_, _ = fmt.Fprintln(os.Stderr, " *", distro)
		}
		return
	}

	// Validate image type

	blueprint := &blueprint.Blueprint{}
	if blueprintArg != "" {
		file, err := ioutil.ReadFile(blueprintArg)
		if err != nil {
			panic("Could not find blueprint: " + err.Error())
		}
		err = json.Unmarshal([]byte(file), &blueprint)
		if err != nil {
			panic("Could not parse blueprint: " + err.Error())
		}
	}

	distros := distro.NewRegistry([]string{"."})
	d := distros.GetDistro(distroArg)
	if d == nil {
		panic("unknown distro: " + distroArg)
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

	rpmmd := rpmmd.NewRPMMD()
	_, checksums, err := rpmmd.Depsolve(packages, nil, d.Repositories(archArg), d.ModulePlatformID(), true)
	if err != nil {
		panic(err.Error())
	}

	pipeline, err := d.Pipeline(blueprint, nil, checksums, archArg, imageType, 0)
	if err != nil {
		panic(err.Error())
	}

	bytes, err := json.Marshal(pipeline)
	if err != nil {
		panic("could not marshal pipeline into JSON")
	}

	os.Stdout.Write(bytes)
}
