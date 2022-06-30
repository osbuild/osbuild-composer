package distro_test

import (
	"fmt"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDistro_Manifest(t *testing.T) {

	distro_test_common.TestDistro_Manifest(
		t,
		"../../test/data/manifests/",
		"*",
		distroregistry.NewDefault(),
		false, // This test case does not check for changes in the imageType package sets!
		"",
		"",
	)
}

var (
	v1manifests = []string{
		`{}`,
		`
{
	"sources": {
		"org.osbuild.files": {
			"urls": {}
		}
	},
	"pipeline": {
		"build": {
			"pipeline": {
				"stages": []
			},
			"runner": "org.osbuild.rhel84"
		},
		"stages": [],
		"assembler": {
			"name": "org.osbuild.qemu",
			"options": {}
		}
	}
}`,
	}

	v2manifests = []string{
		`{"version": "2"}`,
		`
{
	"version": "2",
	"pipelines": [
		{
			"name": "build",
			"runner": "org.osbuild.rhel84",
			"stages": []
		}
	],
	"sources": {
		"org.osbuild.curl": {
			"items": {}
		}
	}
}`,
	}
)

func TestDistro_Version(t *testing.T) {
	require := require.New(t)
	expectedVersion := "1"
	for idx, rawManifest := range v1manifests {
		manifest := distro.Manifest(rawManifest)
		detectedVersion, err := manifest.Version()
		require.NoError(err, "Could not detect Manifest version for %d: %v", idx, err)
		require.Equal(expectedVersion, detectedVersion, "in manifest %d", idx)
	}

	expectedVersion = "2"
	for idx, rawManifest := range v2manifests {
		manifest := distro.Manifest(rawManifest)
		detectedVersion, err := manifest.Version()
		require.NoError(err, "Could not detect Manifest version for %d: %v", idx, err)
		require.Equal(expectedVersion, detectedVersion, "in manifest %d", idx)
	}

	{
		manifest := distro.Manifest("")
		_, err := manifest.Version()
		require.Error(err, "Empty manifest did not return an error")
	}

	{
		manifest := distro.Manifest("{")
		_, err := manifest.Version()
		require.Error(err, "Invalid manifest did not return an error")
	}
}

// Ensure that all package sets defined in the package set chains are defined for the image type
func TestImageType_PackageSetsChains(t *testing.T) {
	distros := distroregistry.NewDefault()
	for _, distroName := range distros.List() {
		d := distros.GetDistro(distroName)
		for _, archName := range d.ListArches() {
			arch, err := d.GetArch(archName)
			require.Nil(t, err)
			for _, imageTypeName := range arch.ListImageTypes() {
				t.Run(fmt.Sprintf("%s/%s/%s", distroName, archName, imageTypeName), func(t *testing.T) {
					imageType, err := arch.GetImageType(imageTypeName)
					require.Nil(t, err)

					imagePkgSets := imageType.PackageSets(blueprint.Blueprint{}, distro.ImageOptions{
						OSTree: ostree.RequestParams{
							URL:    "foo",
							Ref:    "bar",
							Parent: "baz",
						},
					}, nil)
					for packageSetName := range imageType.PackageSetsChains() {
						_, ok := imagePkgSets[packageSetName]
						if !ok {
							// in the new pipeline generation logic the name of the package
							// set chains are taken from the pipelines and do not match the
							// package set names.
							// TODO: redefine package set chains to make this unneccesary
							switch packageSetName {
							case "packages":
								_, ok = imagePkgSets["os"]
								if !ok {
									_, ok = imagePkgSets["ostree-tree"]
								}
							}
						}
						assert.Truef(t, ok, "package set %q defined in a package set chain is not present in the image package sets", packageSetName)
					}
				})
			}
		}
	}
}
