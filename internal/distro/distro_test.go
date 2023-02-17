package distro_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/ostree"
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

					// set up bare minimum args for image type
					var customizations *blueprint.Customizations
					if imageType.Name() == "edge-simplified-installer" || imageType.Name() == "iot-simplified-installer" {
						customizations = &blueprint.Customizations{
							InstallationDevice: "/dev/null",
						}
					}
					bp := blueprint.Blueprint{
						Customizations: customizations,
					}
					options := distro.ImageOptions{
						OSTree: &ostree.ImageOptions{
							URL: "https://example.com", // required by some image types
						},
					}
					manifest, _, err := imageType.Manifest(&bp, options, nil, 0)
					require.NoError(t, err)
					imagePkgSets := manifest.GetPackageSetChains()
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
	type rpmStageOptions struct {
		GPGKeys []string `json:"gpgkeys"`
	}
	type stage struct {
		Type    string          `json:"type"`
		Options rpmStageOptions `json:"options"`
	}
	type pipeline struct {
		Name   string  `json:"name"`
		Stages []stage `json:"stages"`
	}
	type manifest struct {
		Pipelines []pipeline `json:"pipelines"`
	}

	assert := assert.New(t)
	distros := distroregistry.NewDefault()
	for _, distroName := range distros.List() {
		d := distros.GetDistro(distroName)
		for _, archName := range d.ListArches() {
			arch, err := d.GetArch(archName)
			assert.Nil(err)
			for _, imageTypeName := range arch.ListImageTypes() {
				t.Run(fmt.Sprintf("%s/%s/%s", distroName, archName, imageTypeName), func(t *testing.T) {
					imageType, err := arch.GetImageType(imageTypeName)
					assert.Nil(err)

					// set up bare minimum args for image type
					var customizations *blueprint.Customizations
					if imageType.Name() == "edge-simplified-installer" || imageType.Name() == "iot-simplified-installer" {
						customizations = &blueprint.Customizations{
							InstallationDevice: "/dev/null",
						}
					}
					bp := blueprint.Blueprint{
						Customizations: customizations,
					}
					options := distro.ImageOptions{}
					// this repo's gpg keys should get included in the os
					// pipeline's rpm stage
					repos := []rpmmd.RepoConfig{
						{
							Name:        "payload",
							BaseURLs:    []string{"http://payload.example.com"},
							PackageSets: imageType.PayloadPackageSets(),
							GPGKeys:     []string{"payload-gpg-key"},
							CheckGPG:    common.ToPtr(true),
						},
					}
					seed := int64(0)

					// Add ostree options for image types that require them
					options.OSTree = &ostree.ImageOptions{
						URL: "https://example.com",
					}

					// Pipelines that require package sets will fail if none
					// are defined. OS pipelines require a kernel.
					// Add kernel and filesystem to every pipeline so that the
					// manifest creation doesn't fail.
					allPipelines := append(imageType.BuildPipelines(), imageType.PayloadPipelines()...)
					minimalPackageSet := []rpmmd.PackageSpec{
						{Name: "kernel", Checksum: "sha256:a0c936696eb7d5ee3192bf53b9d281cecbb40ca9db520de72cb95817ad92ac72"},
						{Name: "filesystem", Checksum: "sha256:6b4bf18ba28ccbdd49f2716c9f33c9211155ff703fa6c195c78a07bd160da0eb"},
					}

					packageSets := make(map[string][]rpmmd.PackageSpec, len(allPipelines))
					for _, plName := range allPipelines {
						packageSets[plName] = minimalPackageSet
					}

					m, _, err := imageType.Manifest(&bp, options, repos, seed)
					assert.NoError(err)

					containers := make(map[string][]container.Spec, 0)

					ostreeSources := m.GetOSTreeSourceSpecs()
					commits := make(map[string][]ostree.CommitSpec, len(ostreeSources))
					for name, commitSources := range ostreeSources {
						commitSpecs := make([]ostree.CommitSpec, len(commitSources))
						for idx, commitSource := range commitSources {
							commitSpecs[idx] = ostree.CommitSpec{
								Ref:      commitSource.Ref,
								URL:      commitSource.URL,
								Checksum: fmt.Sprintf("%x", sha256.Sum256([]byte(commitSource.URL+commitSource.Ref))),
							}
						}
						commits[name] = commitSpecs
					}
					mf, err := m.Serialize(packageSets, containers, commits)
					assert.NoError(err)
					pm := new(manifest)
					err = json.Unmarshal(mf, pm)
					assert.NoError(err)

					assert.Equal(len(allPipelines), len(pm.Pipelines))
					for idx := range pm.Pipelines {
						// manifest pipeline names should be identical to the ones
						// defined in the image type and in the same order
						assert.Equal(allPipelines[idx], pm.Pipelines[idx].Name)

						if pm.Pipelines[idx].Name == "os" {
							rpmStagePresent := false
							for _, s := range pm.Pipelines[idx].Stages {
								if s.Type == "org.osbuild.rpm" {
									rpmStagePresent = true
									if imageTypeName != "azure-eap7-rhui" {
										// NOTE (akoutsou): Ideally, at some point we will
										// have a good way of reading what's supported by
										// each image type and we can skip or adapt tests
										// based on this information. For image types with
										// a preset workload, payload packages are ignored
										// and dropped and so are the payload
										// repo gpg keys.
										assert.Equal(repos[0].GPGKeys, s.Options.GPGKeys)
									}
								}
							}
							// make sure the gpg keys check was reached
							assert.True(rpmStagePresent)
						}
					}

					// The last pipeline should match the export pipeline.
					// This might change in the future, but for now, let's make
					// sure they match.
					assert.Equal(imageType.Exports()[0], pm.Pipelines[len(pm.Pipelines)-1].Name)

				})
			}
		}
	}
}

// Ensure repositories are assigned to package sets properly.
//
// Each package set should include all the global repositories as well as any
// pipeline/package-set specific repositories.
func TestPipelineRepositories(t *testing.T) {
	require := require.New(t)

	type testCase struct {
		// Repo configs for pipeline generator
		repos []rpmmd.RepoConfig

		// Expected result: map of pipelines to repo names (we only check names for the test).
		// Use the pipeline name * for global repos.
		result map[string][]stringSet
	}

	testCases := map[string]testCase{
		"globalonly": { // only global repos: most common scenario
			repos: []rpmmd.RepoConfig{
				{
					Name:     "global-1",
					BaseURLs: []string{"http://global-1.example.com"},
				},
				{
					Name:     "global-2",
					BaseURLs: []string{"http://global-2.example.com"},
				},
			},
			result: map[string][]stringSet{
				"*": {newStringSet([]string{"global-1", "global-2"})},
			},
		},
		"global+build": { // global repos with build-specific repos: secondary common scenario
			repos: []rpmmd.RepoConfig{
				{
					Name:     "global-11",
					BaseURLs: []string{"http://global-11.example.com"},
				},
				{
					Name:     "global-12",
					BaseURLs: []string{"http://global-12.example.com"},
				},
				{
					Name:        "build-1",
					BaseURLs:    []string{"http://build-1.example.com"},
					PackageSets: []string{"build"},
				},
				{
					Name:        "build-2",
					BaseURLs:    []string{"http://build-2.example.com"},
					PackageSets: []string{"build"},
				},
			},
			result: map[string][]stringSet{
				"*":     {newStringSet([]string{"global-11", "global-12"})},
				"build": {newStringSet([]string{"build-1", "build-2"})},
			},
		},
		"global+os": { // global repos with os-specific repos
			repos: []rpmmd.RepoConfig{
				{
					Name:     "global-21",
					BaseURLs: []string{"http://global-11.example.com"},
				},
				{
					Name:     "global-22",
					BaseURLs: []string{"http://global-12.example.com"},
				},
				{
					Name:        "os-1",
					BaseURLs:    []string{"http://os-1.example.com"},
					PackageSets: []string{"os"},
				},
				{
					Name:        "os-2",
					BaseURLs:    []string{"http://os-2.example.com"},
					PackageSets: []string{"os"},
				},
			},
			result: map[string][]stringSet{
				"*":  {newStringSet([]string{"global-21", "global-22"})},
				"os": {newStringSet([]string{"os-1", "os-2"}), newStringSet([]string{"os-1", "os-2"})},
			},
		},
		"global+os+payload": { // global repos with os-specific repos and (user-defined) payload repositories
			repos: []rpmmd.RepoConfig{
				{
					Name:     "global-21",
					BaseURLs: []string{"http://global-11.example.com"},
				},
				{
					Name:     "global-22",
					BaseURLs: []string{"http://global-12.example.com"},
				},
				{
					Name:        "os-1",
					BaseURLs:    []string{"http://os-1.example.com"},
					PackageSets: []string{"os"},
				},
				{
					Name:        "os-2",
					BaseURLs:    []string{"http://os-2.example.com"},
					PackageSets: []string{"os"},
				},
				{
					Name:     "payload",
					BaseURLs: []string{"http://payload.example.com"},
					// User-defined payload repositories automatically get the "blueprint" key.
					// This is handled by the APIs.
					PackageSets: []string{"blueprint"},
				},
			},
			result: map[string][]stringSet{
				"*": {newStringSet([]string{"global-21", "global-22"})},
				"os": {
					// chain with payload repo only in the second set for the blueprint package depsolve
					newStringSet([]string{"os-1", "os-2"}),
					newStringSet([]string{"os-1", "os-2", "payload"})},
			},
		},
		"noglobal": { // no global repositories; only pipeline restricted ones (unrealistic but technically valid)
			repos: []rpmmd.RepoConfig{
				{
					Name:        "build-1",
					BaseURLs:    []string{"http://build-1.example.com"},
					PackageSets: []string{"build"},
				},
				{
					Name:        "build-2",
					BaseURLs:    []string{"http://build-2.example.com"},
					PackageSets: []string{"build"},
				},
				{
					Name:        "os-1",
					BaseURLs:    []string{"http://os-1.example.com"},
					PackageSets: []string{"os"},
				},
				{
					Name:        "os-2",
					BaseURLs:    []string{"http://os-2.example.com"},
					PackageSets: []string{"os"},
				},
				{
					Name:        "anaconda-1",
					BaseURLs:    []string{"http://anaconda-1.example.com"},
					PackageSets: []string{"anaconda-tree"},
				},
				{
					Name:        "container-1",
					BaseURLs:    []string{"http://container-1.example.com"},
					PackageSets: []string{"container-tree"},
				},
				{
					Name:        "coi-1",
					BaseURLs:    []string{"http://coi-1.example.com"},
					PackageSets: []string{"coi-tree"},
				},
			},
			result: map[string][]stringSet{
				"*":              nil,
				"build":          {newStringSet([]string{"build-1", "build-2"})},
				"os":             {newStringSet([]string{"os-1", "os-2"}), newStringSet([]string{"os-1", "os-2"})},
				"anaconda-tree":  {newStringSet([]string{"anaconda-1"})},
				"container-tree": {newStringSet([]string{"container-1"})},
				"coi-tree":       {newStringSet([]string{"coi-1"})},
			},
		},
		"global+unknown": { // package set names that don't match a pipeline are ignored
			repos: []rpmmd.RepoConfig{
				{
					Name:     "global-1",
					BaseURLs: []string{"http://global-1.example.com"},
				},
				{
					Name:     "global-2",
					BaseURLs: []string{"http://global-2.example.com"},
				},
				{
					Name:        "custom-1",
					BaseURLs:    []string{"http://custom.example.com"},
					PackageSets: []string{"notapipeline"},
				},
			},
			result: map[string][]stringSet{
				"*": {newStringSet([]string{"global-1", "global-2"})},
			},
		},
		"none": { // empty
			repos:  []rpmmd.RepoConfig{},
			result: map[string][]stringSet{},
		},
	}

	distros := distroregistry.NewDefault()
	for tName, tCase := range testCases {
		t.Run(tName, func(t *testing.T) {
			for _, distroName := range distros.List() {
				d := distros.GetDistro(distroName)
				for _, archName := range d.ListArches() {
					arch, err := d.GetArch(archName)
					require.Nil(err)
					for _, imageTypeName := range arch.ListImageTypes() {
						if imageTypeName == "azure-eap7-rhui" {
							// NOTE (akoutsou): Ideally, at some point we will
							// have a good way of reading what's supported by
							// each image type and we can skip or adapt tests
							// based on this information. For image types with
							// a preset workload, payload packages are ignored
							// and dropped.
							continue
						}
						t.Run(fmt.Sprintf("%s/%s/%s", distroName, archName, imageTypeName), func(t *testing.T) {
							imageType, err := arch.GetImageType(imageTypeName)
							require.Nil(err)

							// set up bare minimum args for image type
							var customizations *blueprint.Customizations
							if imageType.Name() == "edge-simplified-installer" || imageType.Name() == "iot-simplified-installer" {
								customizations = &blueprint.Customizations{
									InstallationDevice: "/dev/null",
								}
							}
							bp := blueprint.Blueprint{
								Customizations: customizations,
								Packages: []blueprint.Package{
									{Name: "filesystem"},
								},
							}
							options := distro.ImageOptions{}

							// Add ostree options for image types that require them
							options.OSTree = &ostree.ImageOptions{
								URL: "https://example.com",
							}

							repos := tCase.repos
							manifest, _, err := imageType.Manifest(&bp, options, repos, 0)
							require.NoError(err)
							packageSets := manifest.GetPackageSetChains()

							var globals stringSet
							if len(tCase.result["*"]) > 0 {
								globals = tCase.result["*"][0]
							}
							for psName, psChain := range packageSets {

								expChain := tCase.result[psName]
								if len(expChain) > 0 {
									// if we specified an expected chain it should match the returned.
									if len(expChain) != len(psChain) {
										t.Fatalf("expected %d package sets in the %q chain; got %d", len(expChain), psName, len(psChain))
									}
								} else {
									// if we didn't, initialise to empty before merging globals
									expChain = make([]stringSet, len(psChain))
								}

								for idx := range expChain {
									// merge the globals into each expected set
									expChain[idx] = expChain[idx].Merge(globals)
								}

								for setIdx, set := range psChain {
									// collect repositories in the package set
									repoNamesSet := newStringSet(nil)
									for _, repo := range set.Repositories {
										repoNamesSet.Add(repo.Name)
									}

									// expected set for current package set should be merged with globals
									expected := expChain[setIdx]
									if !repoNamesSet.Equals(expected) {
										t.Errorf("repos for package set %q [idx: %d] %s (distro %q image type %q) do not match expected %s", psName, setIdx, repoNamesSet, d.Name(), imageType.Name(), expected)
									}
								}
							}
						})
					}
				}
			}
		})
	}
}

// a very basic implementation of a Set of strings
type stringSet struct {
	elems map[string]bool
}

func newStringSet(init []string) stringSet {
	s := stringSet{elems: make(map[string]bool)}
	for _, elem := range init {
		s.Add(elem)
	}
	return s
}

func (s stringSet) String() string {
	elemSlice := make([]string, 0, len(s.elems))
	for elem := range s.elems {
		elemSlice = append(elemSlice, elem)
	}
	return "{" + strings.Join(elemSlice, ", ") + "}"
}

func (s stringSet) Add(elem string) {
	s.elems[elem] = true
}

func (s stringSet) Contains(elem string) bool {
	return s.elems[elem]
}

func (s stringSet) Equals(other stringSet) bool {
	if len(s.elems) != len(other.elems) {
		return false
	}

	for elem := range s.elems {
		if !other.Contains(elem) {
			return false
		}
	}

	return true
}

func (s stringSet) Merge(other stringSet) stringSet {
	merged := newStringSet(nil)
	for elem := range s.elems {
		merged.Add(elem)
	}
	for elem := range other.elems {
		merged.Add(elem)
	}
	return merged
}
