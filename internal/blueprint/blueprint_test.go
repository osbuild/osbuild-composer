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
	type fields struct {
		Name        string
		Description string
		Version     string
		Packages    []blueprint.Package
		Modules     []blueprint.Package
		Groups      []blueprint.Package
	}
	type args struct {
		outputFormat string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "ami",
			args: args{"ami"},
			want: "pipelines/ami_empty_blueprint.json",
		},
		{
			name: "ext4",
			args: args{"ext4-filesystem"},
			want: "pipelines/ext4_empty_blueprint.json",
		},
		{
			name: "live-iso",
			args: args{"live-iso"},
			want: "pipelines/liveiso_empty_blueprint.json",
		},
		{
			name: "openstack",
			args: args{"openstack"},
			want: "pipelines/openstack_empty_blueprint.json",
		},
		{
			name: "partitioned-disk",
			args: args{"partitioned-disk"},
			want: "pipelines/disk_empty_blueprint.json",
		},
		{
			name: "qcow2",
			args: args{"qcow2"},
			want: "pipelines/qcow2_empty_blueprint.json",
		},
		{
			name: "tar",
			args: args{"tar"},
			want: "pipelines/tar_empty_blueprint.json",
		},
		{
			name: "vhd",
			args: args{"vhd"},
			want: "pipelines/vhd_empty_blueprint.json",
		},
		{
			name: "vmdk",
			args: args{"vmdk"},
			want: "pipelines/vmdk_empty_blueprint.json",
		},
		{
			name:    "invalid-output-type",
			args:    args{"foobar"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &blueprint.Blueprint{
				Name:        tt.fields.Name,
				Description: tt.fields.Description,
				Version:     tt.fields.Version,
				Packages:    tt.fields.Packages,
				Modules:     tt.fields.Modules,
				Groups:      tt.fields.Groups,
			}
			file, _ := ioutil.ReadFile(tt.want)
			var want pipeline.Pipeline
			json.Unmarshal([]byte(file), &want)
			got, err := b.ToPipeline(tt.args.outputFormat)
			if (err != nil) != tt.wantErr {
				t.Errorf("Blueprint.ToPipeline() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got, &want) {
					t.Errorf("Blueprint.ToPipeline() = %v, want %v", got, &want)
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
