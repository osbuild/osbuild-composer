package distro_test_common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const RandomTestSeed = 0

func TestDistro_Manifest(t *testing.T, pipelinePath string, prefix string, distros ...distro.Distro) {
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
			Distro       string               `json:"distro"`
			Arch         string               `json:"arch"`
			ImageType    string               `json:"image-type"`
			Repositories []repository         `json:"repositories"`
			Blueprint    *blueprint.Blueprint `json:"blueprint"`
		}
		type rpmMD struct {
			BuildPackages []rpmmd.PackageSpec `json:"build-packages"`
			Packages      []rpmmd.PackageSpec `json:"packages"`
		}
		var tt struct {
			ComposeRequest *composeRequest `json:"compose-request"`
			RpmMD          *rpmMD          `json:"rpmmd"`
			Manifest       distro.Manifest `json:"manifest,omitempty"`
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
			distros, err := distro.NewRegistry(distros...)
			require.NoError(t, err)
			d := distros.GetDistro(tt.ComposeRequest.Distro)
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
			got, err := imageType.Manifest(tt.ComposeRequest.Blueprint.Customizations,
				distro.ImageOptions{
					Size: imageType.Size(0),
					OSTree: distro.OSTreeImageOptions{
						Ref: imageType.OSTreeRef(),
					},
				},
				repos,
				tt.RpmMD.Packages,
				tt.RpmMD.BuildPackages,
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

func isOSTree(imgType distro.ImageType) bool {
	for _, pkg := range imgType.BuildPackages() {
		if pkg == "rpm-ostree" {
			return true
		}
	}
	return false
}

var knownKernels = []string{"kernel", "kernel-debug", "kernel-rt"}

// Returns the number of known kernels in the package list
func kernelCount(imgType distro.ImageType) int {
	pkgs, _ := imgType.Packages(blueprint.Blueprint{})
	n := 0
	for _, pkg := range pkgs {
		for _, kernel := range knownKernels {
			if kernel == pkg {
				n++
			}
		}
	}
	return n
}

func TestDistro_KernelOption(t *testing.T, d distro.Distro) {
	for _, archName := range d.ListArches() {
		arch, err := d.GetArch(archName)
		assert.NoError(t, err)
		for _, typeName := range arch.ListImageTypes() {
			imgType, err := arch.GetImageType(typeName)
			assert.NoError(t, err)
			nk := kernelCount(imgType)
			// at least one kernel for general image types
			// exactly one kernel for OSTree commits
			if nk < 1 || (isOSTree(imgType) && nk != 1) {
				assert.Fail(t, fmt.Sprintf("%s Kernel count", d.Name()),
					"Image type %s (arch %s) specifies %d Kernel packages", typeName, archName, nk)
			}
		}
	}
}
