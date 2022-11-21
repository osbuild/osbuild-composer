package distro_test_common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/dnfjson"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const RandomTestSeed = 0

func TestDistro_Manifest(t *testing.T, pipelinePath string, prefix string, registry *distroregistry.Registry, depsolvePkgSets bool, dnfCacheDir, dnfJsonPath string) {
	assert := assert.New(t)
	fileNames, err := filepath.Glob(filepath.Join(pipelinePath, prefix))
	assert.NoErrorf(err, "Could not read pipelines directory '%s': %v", pipelinePath, err)
	require.Greaterf(t, len(fileNames), 0, "No pipelines found in %s for %s", pipelinePath, prefix)
	for _, fileName := range fileNames {
		type repository struct {
			BaseURL    string `json:"baseurl,omitempty"`
			Metalink   string `json:"metalink,omitempty"`
			MirrorList string `json:"mirrorlist,omitempty"`
			GPGKey     string `json:"gpgkey,omitempty"`
			CheckGPG   bool   `json:"check_gpg,omitempty"`
		}
		type composeRequest struct {
			Distro       string                `json:"distro"`
			Arch         string                `json:"arch"`
			ImageType    string                `json:"image-type"`
			Repositories []repository          `json:"repositories"`
			Blueprint    *blueprint.Blueprint  `json:"blueprint"`
			OSTree       *ostree.RequestParams `json:"ostree"`
		}
		var tt struct {
			ComposeRequest  *composeRequest                `json:"compose-request"`
			PackageSpecSets map[string][]rpmmd.PackageSpec `json:"rpmmd"`
			Manifest        distro.Manifest                `json:"manifest,omitempty"`
			Containers      []container.Spec               `json:"containers,omitempty"`
		}
		file, err := ioutil.ReadFile(fileName)
		assert.NoErrorf(err, "Could not read test-case '%s': %v", fileName, err)
		err = json.Unmarshal([]byte(file), &tt)
		assert.NoErrorf(err, "Could not parse test-case '%s': %v", fileName, err)
		if tt.ComposeRequest == nil || tt.ComposeRequest.Blueprint == nil {
			t.Logf("Skipping '%s'.", fileName)
			continue
		}

		repos := make([]rpmmd.RepoConfig, len(tt.ComposeRequest.Repositories))
		for i, repo := range tt.ComposeRequest.Repositories {
			repos[i] = rpmmd.RepoConfig{
				Name:       fmt.Sprintf("repo-%d", i),
				BaseURL:    repo.BaseURL,
				Metalink:   repo.Metalink,
				MirrorList: repo.MirrorList,
				GPGKey:     repo.GPGKey,
				CheckGPG:   repo.CheckGPG,
			}
		}
		t.Run(path.Base(fileName), func(t *testing.T) {
			require.NoError(t, err)
			d := registry.GetDistro(tt.ComposeRequest.Distro)
			if d == nil {
				t.Errorf("unknown distro: %v", tt.ComposeRequest.Distro)
				return
			}
			arch, err := d.GetArch(tt.ComposeRequest.Arch)
			if err != nil {
				t.Errorf("unknown arch: %v", tt.ComposeRequest.Arch)
				return
			}
			imageType, err := arch.GetImageType(tt.ComposeRequest.ImageType)
			if err != nil {
				t.Errorf("unknown image type: %v", tt.ComposeRequest.ImageType)
				return
			}

			var ostreeOptions distro.OSTreeImageOptions
			if ref := imageType.OSTreeRef(); ref != "" {
				if tt.ComposeRequest.OSTree != nil {
					ostreeOptions = distro.OSTreeImageOptions{
						ImageRef:      tt.ComposeRequest.OSTree.Ref,
						FetchChecksum: tt.ComposeRequest.OSTree.Parent,
						URL:           tt.ComposeRequest.OSTree.URL,
					}
				}
			}
			if ostreeOptions.ImageRef == "" { // set image type default if not specified in request
				ostreeOptions.ImageRef = imageType.OSTreeRef()
			}

			options := distro.ImageOptions{
				Size:   imageType.Size(0),
				OSTree: ostreeOptions,
			}

			var imgPackageSpecSets map[string][]rpmmd.PackageSpec
			// depsolve the image's package set to catch changes in the image's default package set.
			// downside is that this takes long time
			if depsolvePkgSets {
				require.NotEmptyf(t, dnfCacheDir, "DNF cache directory path must be provided when chosen to depsolve image package sets")
				require.NotEmptyf(t, dnfJsonPath, "path to 'dnf-json' must be provided when chosen to depsolve image package sets")
				imgPackageSpecSets = getImageTypePkgSpecSets(
					imageType,
					*tt.ComposeRequest.Blueprint,
					options,
					repos,
					dnfCacheDir,
					dnfJsonPath,
				)
			} else {
				imgPackageSpecSets = tt.PackageSpecSets
			}

			got, err := imageType.Manifest(tt.ComposeRequest.Blueprint.Customizations,
				options,
				repos,
				imgPackageSpecSets,
				tt.Containers,
				RandomTestSeed)

			if (err == nil && tt.Manifest == nil) || (err != nil && tt.Manifest != nil) {
				t.Errorf("distro.Manifest() error = %v", err)
				return
			}
			if tt.Manifest != nil {
				var expected, actual interface{}
				err = json.Unmarshal(tt.Manifest, &expected)
				require.NoError(t, err)
				err = json.Unmarshal(got, &actual)
				require.NoError(t, err)

				diff := cmp.Diff(expected, actual)
				require.Emptyf(t, diff, "Distro: %s\nArch: %s\nImage type: %s\nTest case file: %s\n", d.Name(), arch.Name(), imageType.Name(), fileName)
			}
		})
	}
}

func getImageTypePkgSpecSets(imageType distro.ImageType, bp blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig, cacheDir, dnfJsonPath string) map[string][]rpmmd.PackageSpec {
	imgPackageSets := imageType.PackageSets(bp, options, repos)

	solver := dnfjson.NewSolver(imageType.Arch().Distro().ModulePlatformID(), imageType.Arch().Distro().Releasever(), imageType.Arch().Name(), cacheDir)
	depsolvedSets := make(map[string][]rpmmd.PackageSpec)
	for name, packages := range imgPackageSets {
		res, err := solver.Depsolve(packages)
		if err != nil {
			panic("Could not depsolve: " + err.Error())
		}
		depsolvedSets[name] = res
	}

	return depsolvedSets
}

func isOSTree(imgType distro.ImageType) bool {
	return imgType.OSTreeRef() != ""
}

var knownKernels = []string{"kernel", "kernel-debug", "kernel-rt"}

// Returns the number of known kernels in the package list
func kernelCount(imgType distro.ImageType, bp blueprint.Blueprint) int {
	sets := imgType.PackageSets(bp, distro.ImageOptions{}, nil)

	// Use a map to count unique kernels in a package set. If the same kernel
	// name appears twice, it will only be installed once, so we only count it
	// once.
	kernels := make(map[string]bool)
	for _, name := range []string{
		// payload package set names
		"os", "ostree-tree", "anaconda-tree",
		"packages", "installer",
	} {
		for _, pset := range sets[name] {
			for _, pkg := range pset.Include {
				for _, kernel := range knownKernels {
					if kernel == pkg {
						kernels[kernel] = true
					}
				}
			}
			if len(kernels) > 0 {
				// BUG: some RHEL image types contain both 'packages'
				// and 'installer' even though only 'installer' is used
				// this counts the kernel package twice. None of these
				// sets should appear more than once, so return the count
				// for the first package set that has at least one kernel.
				return len(kernels)
			}
		}
	}
	return len(kernels)
}

func TestDistro_KernelOption(t *testing.T, d distro.Distro) {
	skipList := map[string]bool{
		// Ostree installers and raw images download a payload to embed or
		// deploy.  The kernel is part of the payload so it doesn't appear in
		// the image type's package lists.
		"iot-installer":             true,
		"edge-installer":            true,
		"edge-simplified-installer": true,
		"iot-raw-image":             true,
		"edge-raw-image":            true,

		// the tar image type is a minimal image type which is not expected to
		// be usable without a blueprint (see commit 83a63aaf172f556f6176e6099ffaa2b5357b58f5).
		"tar": true,

		// containers don't have kernels
		"container": true,

		// image installer on Fedora doesn't support kernel customizations
		// on RHEL we support kernel name
		// TODO: Remove when we unify the allowed options
		"image-installer": true,
	}

	{ // empty blueprint: all image types should just have the default kernel
		for _, archName := range d.ListArches() {
			arch, err := d.GetArch(archName)
			assert.NoError(t, err)
			for _, typeName := range arch.ListImageTypes() {
				if true {
					break
				}
				if skipList[typeName] {
					continue
				}
				imgType, err := arch.GetImageType(typeName)
				assert.NoError(t, err)
				nk := kernelCount(imgType, blueprint.Blueprint{})

				if nk != 1 {
					assert.Fail(t, fmt.Sprintf("%s Kernel count", d.Name()),
						"Image type %s (arch %s) specifies %d Kernel packages", typeName, archName, nk)
				}
			}
		}
	}

	{ // kernel in blueprint: the specified kernel replaces the default
		for _, kernelName := range []string{"kernel", "kernel-debug"} {
			bp := blueprint.Blueprint{
				Customizations: &blueprint.Customizations{
					Kernel: &blueprint.KernelCustomization{
						Name: kernelName,
					},
				},
			}
			for _, archName := range d.ListArches() {
				arch, err := d.GetArch(archName)
				assert.NoError(t, err)
				for _, typeName := range arch.ListImageTypes() {
					if typeName != "image-installer" {
						continue
					}
					if skipList[typeName] {
						continue
					}
					imgType, err := arch.GetImageType(typeName)
					assert.NoError(t, err)
					nk := kernelCount(imgType, bp)

					// ostree image types should have only one kernel
					// other image types should have at least 1
					if nk < 1 || (nk != 1 && isOSTree(imgType)) {
						assert.Fail(t, fmt.Sprintf("%s Kernel count", d.Name()),
							"Image type %s (arch %s) specifies %d Kernel packages", typeName, archName, nk)
					}
				}
			}
		}
	}
}

// GetTestingPackageSpecSets returns PackageSpecSets useful for unit testing.
//
// A dummy PackageSpec for the provided packageName is added
// to all PackageSpecSets provided in pkgSetNames.
//
// E.g. `kernel` package is a hard requirement of some payload pipelines
// and they panic if it is not found in the packageSpecSets passed to
// Manifest().
func GetTestingPackageSpecSets(packageName, arch string, pkgSetNames []string) map[string][]rpmmd.PackageSpec {
	pkgTestingSpec := []rpmmd.PackageSpec{
		{
			Name:           packageName,
			Epoch:          0,
			Version:        "1.2.3",
			Release:        "2.el123",
			Arch:           arch,
			RemoteLocation: "http://example.org",
			Checksum:       "lorenipsum",
			Secrets:        "lorenipsum",
			CheckGPG:       false,
		},
	}
	testPackageSpecSets := map[string][]rpmmd.PackageSpec{}
	for _, pkgSetName := range pkgSetNames {
		testPackageSpecSets[pkgSetName] = pkgTestingSpec
	}
	return testPackageSpecSets
}

// GetTestingImagePackageSpecSets returns PackageSpecSets for all package sets
// defined by the provided ImageType, which is useful for unit testing.
func GetTestingImagePackageSpecSets(packageName string, i distro.ImageType) map[string][]rpmmd.PackageSpec {
	arch := i.Arch().Name()
	imagePackageSets := make([]string, 0, len(i.PackageSets(blueprint.Blueprint{}, distro.ImageOptions{
		OSTree: distro.OSTreeImageOptions{
			URL:           "foo",
			ImageRef:      "bar",
			FetchChecksum: "baz",
		},
	}, nil)))
	for pkgSetName := range i.PackageSets(blueprint.Blueprint{}, distro.ImageOptions{
		OSTree: distro.OSTreeImageOptions{
			URL:           "foo",
			ImageRef:      "bar",
			FetchChecksum: "baz",
		},
	}, nil) {
		imagePackageSets = append(imagePackageSets, pkgSetName)
	}
	return GetTestingPackageSpecSets(packageName, arch, imagePackageSets)
}
