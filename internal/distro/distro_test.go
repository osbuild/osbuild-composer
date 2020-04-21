package distro_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora30"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel81"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel82"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel83"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func TestDistro_Manifest(t *testing.T) {
	pipelinePath := "../../test/cases/"
	fileInfos, err := ioutil.ReadDir(pipelinePath)
	assert.NoErrorf(t, err, "Could not read pipelines directory '%s'", pipelinePath)

	for _, fileInfo := range fileInfos {
		type repository struct {
			BaseURL    string `json:"baseurl,omitempty"`
			Metalink   string `json:"metalink,omitempty"`
			MirrorList string `json:"mirrorlist,omitempty"`
			GPGKey     string `json:"gpgkey,omitempty"`
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
			ComposeRequest *composeRequest   `json:"compose-request"`
			RpmMD          *rpmMD            `json:"rpmmd"`
			Manifest       *osbuild.Manifest `json:"manifest,omitempty"`
		}
		file, err := ioutil.ReadFile(pipelinePath + fileInfo.Name())
		assert.NoErrorf(t, err, "Could not read test-case '%s'", fileInfo.Name())

		err = json.Unmarshal([]byte(file), &tt)
		assert.NoErrorf(t, err, "Could not parse test-case '%s'", fileInfo.Name())

		if tt.ComposeRequest == nil || tt.ComposeRequest.Blueprint == nil {
			t.Logf("Skipping '%s'.", fileInfo.Name())
			continue
		}

		repos := make([]rpmmd.RepoConfig, len(tt.ComposeRequest.Repositories))
		for i, repo := range tt.ComposeRequest.Repositories {
			repos[i] = rpmmd.RepoConfig{
				Id:         fmt.Sprintf("repo-%d", i),
				BaseURL:    repo.BaseURL,
				Metalink:   repo.Metalink,
				MirrorList: repo.MirrorList,
				GPGKey:     repo.GPGKey,
			}
		}
		t.Run(tt.ComposeRequest.ImageType, func(t *testing.T) {
			distros, err := distro.NewRegistry(fedora30.New(), fedora31.New(), fedora32.New(), rhel81.New(), rhel82.New(), rhel83.New())
			if err != nil {
				t.Fatal(err)
			}
			d := distros.GetDistro(tt.ComposeRequest.Distro)
			require.NotNilf(t, d, "unknown distro: %v", tt.ComposeRequest.Distro)

			arch, err := d.GetArch(tt.ComposeRequest.Arch)
			require.NoErrorf(t, err, "unknown arch: %v", tt.ComposeRequest.Arch)

			imageType, err := arch.GetImageType(tt.ComposeRequest.ImageType)
			require.NoError(t, err, "unknown image type: %v", tt.ComposeRequest.ImageType)

			got, err := imageType.Manifest(tt.ComposeRequest.Blueprint.Customizations,
				repos,
				tt.RpmMD.Packages,
				tt.RpmMD.BuildPackages,
				imageType.Size(0))
			if (err != nil) != (tt.Manifest == nil) {
				t.Errorf("distro.Manifest() error = %v", err)
				return
			}
			if tt.Manifest != nil {
				assert.Equalf(t, tt.Manifest, got, "d.Manifest() different from expected")
			}
		})
	}
}

// Test that all distros are registered properly and that Registry.List() works.
func TestDistro_RegistryList(t *testing.T) {
	expected := []string{
		"fedora-30",
		"fedora-31",
		"fedora-32",
		"rhel-8.1",
		"rhel-8.2",
		"rhel-8.3",
	}

	distros, err := distro.NewRegistry(fedora30.New(), fedora31.New(), fedora32.New(), rhel81.New(), rhel82.New(), rhel83.New())
	require.NoError(t, err)

	require.Equalf(t, expected, distros.List(), "unexpected list of distros")
}
