// Simple tool to dump a JSON object containing all package sets for a specific
// distro x arch x image type.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/ostree"
)

func main() {
	var distroName string
	var archName string
	var imageName string

	flag.StringVar(&distroName, "distro", "", "Distribution name")
	flag.StringVar(&archName, "arch", "", "Architecture name")
	flag.StringVar(&imageName, "image", "", "Image name")
	flag.Parse()

	if distroName == "" || archName == "" || imageName == "" {
		flag.Usage()
		os.Exit(1)
	}

	dr := distroregistry.NewDefault()

	d := dr.GetDistro(distroName)
	if d == nil {
		panic(fmt.Errorf("Distro %q does not exist", distroName))
	}

	arch, err := d.GetArch(archName)
	if err != nil {
		panic(err)
	}

	image, err := arch.GetImageType(imageName)
	if err != nil {
		panic(err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	options := distro.ImageOptions{
		OSTree: &ostree.ImageOptions{
			URL: "https://example.com", // required by some image types
		},
	}
	manifest, _, err := image.Manifest(&blueprint.Blueprint{}, options, nil, 0)
	if err != nil {
		panic(err)
	}
	_ = encoder.Encode(manifest.Content.PackageSets)
}
