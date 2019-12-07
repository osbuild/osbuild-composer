package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func main() {
	var format string
	var blueprintArg string
	var distroArg string
	flag.StringVar(&format, "output-format", "qcow2", "output format")
	flag.StringVar(&blueprintArg, "blueprint", "", "blueprint to translate")
	flag.StringVar(&distroArg, "distro", "", "distribution to create")
	flag.Parse()

	blueprint := &blueprint.Blueprint{}
	if blueprintArg != "" {
		file, err := ioutil.ReadFile(blueprintArg)
		if err != nil {
			panic("Colud not find blueprint")
		}
		err = json.Unmarshal([]byte(file), &blueprint)
		if err != nil {
			panic("Colud not parse blueprint")
		}
	}

	d := distro.New(distroArg)
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
	_, checksums, err := rpmmd.Depsolve(packages, d.Repositories())
	if err != nil {
		panic(err.Error())
	}

	pipeline, err := d.Pipeline(blueprint, checksums, format)
	if err != nil {
		panic(err.Error())
	}

	bytes, err := json.Marshal(pipeline)
	if err != nil {
		panic("could not marshal pipeline into JSON")
	}

	os.Stdout.Write(bytes)
}
