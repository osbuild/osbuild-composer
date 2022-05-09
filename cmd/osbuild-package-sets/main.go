// Simple tool to dump a JSON object containing all package sets for a specific
// distro x arch x image type.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
)

func main() {
	var distroName string
	var archName string
	var imageName string

	flag.StringVar(&distroName, "distro", "", "Distribution name")
	flag.StringVar(&archName, "arch", "", "Architecture name")
	flag.StringVar(&imageName, "image", "", "Image name")
	flag.Parse()

	dr := distroregistry.NewDefault()

	distro := dr.GetDistro(distroName)
	if distro == nil {
		panic(fmt.Errorf("Distro %q does not exist", distro))
	}

	arch, err := distro.GetArch(archName)
	if err != nil {
		panic(err)
	}

	image, err := arch.GetImageType(imageName)
	if err != nil {
		panic(err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	pkgset := image.PackageSets(blueprint.Blueprint{}, nil)
	_ = encoder.Encode(pkgset)
}
