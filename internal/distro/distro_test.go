package distro_test

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora30"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel81"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel82"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func TestDistro_Manifest(t *testing.T) {
	pipelinePath := "../../test/cases/"
	fileInfos, err := ioutil.ReadDir(pipelinePath)
	if err != nil {
		t.Errorf("Could not read pipelines directory '%s': %v", pipelinePath, err)
	}
	for _, fileInfo := range fileInfos {
		type composeRequest struct {
			Distro       string               `json:"distro"`
			Arch         string               `json:"arch"`
			OutputFormat string               `json:"output-format"`
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
		if err != nil {
			t.Errorf("Could not read test-case '%s': %v", fileInfo.Name(), err)
		}
		err = json.Unmarshal([]byte(file), &tt)
		if err != nil {
			t.Errorf("Could not parse test-case '%s': %v", fileInfo.Name(), err)
		}
		if tt.ComposeRequest == nil || tt.ComposeRequest.Blueprint == nil {
			t.Logf("Skipping '%s'.", fileInfo.Name())
			continue
		}
		repoMap, err := rpmmd.LoadRepositories([]string{"../.."}, tt.ComposeRequest.Distro)
		if err != nil {
			t.Fatalf("rpmmd.LoadRepositories: %v", err)
		}
		t.Run(tt.ComposeRequest.OutputFormat, func(t *testing.T) {
			distros, err := distro.NewRegistry(fedora30.New(), fedora31.New(), fedora32.New(), rhel81.New(), rhel82.New())
			if err != nil {
				t.Fatal(err)
			}
			d := distros.GetDistro(tt.ComposeRequest.Distro)
			if d == nil {
				t.Errorf("unknown distro: %v", tt.ComposeRequest.Distro)
				return
			}
			size := d.GetSizeForOutputType(tt.ComposeRequest.OutputFormat, 0)
			got, err := d.Manifest(tt.ComposeRequest.Blueprint.Customizations,
				repoMap[tt.ComposeRequest.Arch],
				tt.RpmMD.Packages,
				tt.RpmMD.BuildPackages,
				tt.ComposeRequest.Arch,
				tt.ComposeRequest.OutputFormat,
				size)
			if (err != nil) != (tt.Manifest == nil) {
				t.Errorf("distro.Manifest() error = %v", err)
				return
			}
			if tt.Manifest != nil {
				if diff := cmp.Diff(got, tt.Manifest); diff != "" {
					t.Errorf("d.Manifest() different from expected: %v", diff)
				}
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
	}

	distros, err := distro.NewRegistry(fedora30.New(), fedora31.New(), fedora32.New(), rhel81.New(), rhel82.New())
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(distros.List(), expected); diff != "" {
		t.Errorf("unexpected list of distros: %v", diff)
	}
}
