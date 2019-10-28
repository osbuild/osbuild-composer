package blueprint_test

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

func TestInvalidOutputFormatError_Error(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "basic",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &blueprint.InvalidOutputFormatError{}
			if got := e.Error(); got != tt.want {
				t.Errorf("InvalidOutputFormatError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListOutputFormats(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{
			name: "basic",
			want: []string{
				"ami",
				"ext4-filesystem",
				"live-iso",
				"openstack",
				"partitioned-disk",
				"qcow2",
				"tar",
				"vhd",
				"vmdk",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := blueprint.ListOutputFormats(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListOutputFormats() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlueprint_ToPipeline(t *testing.T) {
	pipelinePath := "../../tools/test_image_info/pipelines/"
	fileInfos, err := ioutil.ReadDir(pipelinePath)
	if err != nil {
		t.Errorf("Could not read pipelines directory '%s': %v", pipelinePath, err)
	}
	for _, fileInfo := range fileInfos {
		type compose struct {
			OutputFormat string               `json:"output-format"`
			Blueprint    *blueprint.Blueprint `json:"blueprint"`
		}
		var tt struct {
			Compose  *compose           `json:"compose"`
			Pipeline *pipeline.Pipeline `json:"pipeline,omitempty"`
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
			got, err := tt.Compose.Blueprint.ToPipeline(tt.Compose.OutputFormat)
			if (err != nil) != (tt.Pipeline == nil) {
				t.Errorf("Blueprint.ToPipeline() error = %v", err)
				return
			}
			if tt.Pipeline != nil {
				if !reflect.DeepEqual(got, tt.Pipeline) {
					// Without this the "difference" is just a list of pointers.
					gotJson, _ := json.Marshal(got)
					fileJson, _ := json.Marshal(tt.Pipeline)
					t.Errorf("Blueprint.ToPipeline() =\n%v,\nwant =\n%v", string(gotJson), string(fileJson))
				}
			}
		})
	}
}

func TestFilenameFromType(t *testing.T) {
	type args struct {
		outputFormat string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name:  "ami",
			args:  args{"ami"},
			want:  "image.ami",
			want1: "application/x-qemu-disk",
		},
		{
			name:  "ext4",
			args:  args{"ext4-filesystem"},
			want:  "image.img",
			want1: "application/octet-stream",
		},
		{
			name:  "live-iso",
			args:  args{"live-iso"},
			want:  "image.iso",
			want1: "application/x-iso9660-image",
		},
		{
			name:  "openstack",
			args:  args{"openstack"},
			want:  "image.qcow2",
			want1: "application/x-qemu-disk",
		},
		{
			name:  "partitioned-disk",
			args:  args{"partitioned-disk"},
			want:  "image.img",
			want1: "application/octet-stream",
		},
		{
			name:  "qcow2",
			args:  args{"qcow2"},
			want:  "image.qcow2",
			want1: "application/x-qemu-disk",
		},
		{
			name:  "tar",
			args:  args{"tar"},
			want:  "image.tar",
			want1: "application/x-tar",
		},
		{
			name:  "vhd",
			args:  args{"vhd"},
			want:  "image.vhd",
			want1: "application/x-vhd",
		},
		{
			name:  "vmdk",
			args:  args{"vmdk"},
			want:  "image.vmdk",
			want1: "application/x-vmdk",
		},
		{
			name:    "invalid-output-type",
			args:    args{"foobar"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := blueprint.FilenameFromType(tt.args.outputFormat)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilenameFromType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got != tt.want {
					t.Errorf("FilenameFromType() got = %v, want %v", got, tt.want)
				}
				if got1 != tt.want1 {
					t.Errorf("FilenameFromType() got1 = %v, want %v", got1, tt.want1)
				}
			}
		})
	}
}
