package distro_test

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
)

func TestDistro_Pipeline(t *testing.T) {
	pipelinePath := "../../test/cases/"
	fileInfos, err := ioutil.ReadDir(pipelinePath)
	if err != nil {
		t.Errorf("Could not read pipelines directory '%s': %v", pipelinePath, err)
	}
	for _, fileInfo := range fileInfos {
		type compose struct {
			Distro       string               `json:"distro"`
			Arch         string               `json:"arch"`
			OutputFormat string               `json:"output-format"`
			Checksums    map[string]string    `json:"checksums"`
			Blueprint    *blueprint.Blueprint `json:"blueprint"`
		}
		var tt struct {
			Compose  *compose          `json:"compose"`
			Pipeline *osbuild.Pipeline `json:"pipeline,omitempty"`
		}
		file, err := ioutil.ReadFile(pipelinePath + fileInfo.Name())
		if err != nil {
			t.Errorf("Colud not read test-case '%s': %v", fileInfo.Name(), err)
		}
		err = json.Unmarshal([]byte(file), &tt)
		if err != nil {
			t.Errorf("Colud not parse test-case '%s': %v", fileInfo.Name(), err)
		}
		if tt.Compose == nil || tt.Compose.Blueprint == nil {
			t.Logf("Skipping '%s'.", fileInfo.Name())
			continue
		}
		t.Run(tt.Compose.OutputFormat, func(t *testing.T) {
			distros := distro.NewRegistry([]string{"../.."})
			d := distros.GetDistro(tt.Compose.Distro)
			if d == nil {
				t.Errorf("unknown distro: %v", tt.Compose.Distro)
				return
			}
			size := d.GetSizeForOutputType(tt.Compose.OutputFormat, 0)
			got, err := d.Pipeline(tt.Compose.Blueprint, nil, nil, nil, tt.Compose.Checksums, tt.Compose.Arch, tt.Compose.OutputFormat, size)
			if (err != nil) != (tt.Pipeline == nil) {
				t.Errorf("distro.Pipeline() error = %v", err)
				return
			}
			if tt.Pipeline != nil {
				if !reflect.DeepEqual(got, tt.Pipeline) {
					// Without this the "difference" is just a list of pointers.
					gotJson, _ := json.Marshal(got)
					fileJson, _ := json.Marshal(tt.Pipeline)
					t.Errorf("d.Pipeline() =\n%v,\nwant =\n%v", string(gotJson), string(fileJson))
				}
			}
		})
	}
}
