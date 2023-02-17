package distro_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
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
						OSTree: distro.OSTreeImageOptions{
							URL:           "foo",
							ImageRef:      "bar",
							FetchChecksum: "baz",
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

// Ensure all image types report the correct names for their pipelines.
// Each image type contains a list of build and payload pipelines. They are
// needed for knowing the names of pipelines from the static object without
// having access to a manifest, which we need when parsing metadata from build
// results.
func TestImageTypePipelineNames(t *testing.T) {
	// types for parsing the opaque manifest with just the fields we care about
	type pipeline struct {
		Name string `json:"name"`
	}
	type manifest struct {
		Pipelines []pipeline `json:"pipelines"`
	}

	require := require.New(t)
	distros := distroregistry.NewDefault()
	for _, distroName := range distros.List() {
		d := distros.GetDistro(distroName)
		for _, archName := range d.ListArches() {
			arch, err := d.GetArch(archName)
			require.Nil(err)
			for _, imageTypeName := range arch.ListImageTypes() {
				t.Run(fmt.Sprintf("%s/%s/%s", distroName, archName, imageTypeName), func(t *testing.T) {
					imageType, err := arch.GetImageType(imageTypeName)
					require.Nil(err)

					// set up bare minimum args for image type
					customizations := &blueprint.Customizations{}
					if imageType.Name() == "edge-simplified-installer" {
						customizations = &blueprint.Customizations{
							InstallationDevice: "/dev/null",
						}
					}
					bp := blueprint.Blueprint{
						Customizations: customizations,
					}
					options := distro.ImageOptions{}
					repos := make([]rpmmd.RepoConfig, 0)
					containers := make([]container.Spec, 0)
					seed := int64(0)

					// Add ostree options for image types that require them
					options.OSTree = distro.OSTreeImageOptions{
						ImageRef:      imageType.OSTreeRef(),
						URL:           "https://example.com/repo",
						FetchChecksum: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
					}

					// Pipelines that require package sets will fail if none
					// are defined. OS pipelines require a kernel.
					// Add kernel and filesystem to every pipeline so that the
					// manifest creation doesn't fail.
					allPipelines := append(imageType.BuildPipelines(), imageType.PayloadPipelines()...)
					minimalPackageSet := []rpmmd.PackageSpec{
						{Name: "kernel"},
						{Name: "filesystem"},
					}

					packageSets := make(map[string][]rpmmd.PackageSpec, len(allPipelines))
					for _, plName := range allPipelines {
						packageSets[plName] = minimalPackageSet
					}

					m, err := imageType.Manifest(bp.Customizations, options, repos, packageSets, containers, seed)
					require.NoError(err)
					pm := new(manifest)
					err = json.Unmarshal(m, pm)
					require.NoError(err)

					require.Equal(len(allPipelines), len(pm.Pipelines))
					for idx := range pm.Pipelines {
						// manifest pipeline names should be identical to the ones
						// defined in the image type and in the same order
						require.Equal(allPipelines[idx], pm.Pipelines[idx].Name)
					}

					// The last pipeline should match the export pipeline.
					// This might change in the future, but for now, let's make
					// sure they match.
					require.Equal(imageType.Exports()[0], pm.Pipelines[len(pm.Pipelines)-1].Name)

				})
			}
		}
	}
}
