package fedora31_test

import (
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
)

func TestListOutputFormats(t *testing.T) {
	want := []string{
		"ami",
		"ext4-filesystem",
		"openstack",
		"partitioned-disk",
		"qcow2",
		"tar",
		"vhd",
		"vmdk",
	}

	f31 := fedora31.New()

	if got := f31.ListOutputFormats(); !reflect.DeepEqual(got, want) {
		t.Errorf("ListOutputFormats() = %v, want %v", got, want)
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
			want:  "image.raw.xz",
			want1: "application/octet-stream",
		},
		{
			name:  "ext4",
			args:  args{"ext4-filesystem"},
			want:  "filesystem.img",
			want1: "application/octet-stream",
		},
		{
			name:  "openstack",
			args:  args{"openstack"},
			want:  "disk.qcow2",
			want1: "application/x-qemu-disk",
		},
		{
			name:  "partitioned-disk",
			args:  args{"partitioned-disk"},
			want:  "disk.img",
			want1: "application/octet-stream",
		},
		{
			name:  "qcow2",
			args:  args{"qcow2"},
			want:  "disk.qcow2",
			want1: "application/x-qemu-disk",
		},
		{
			name:  "tar",
			args:  args{"tar"},
			want:  "root.tar.xz",
			want1: "application/x-tar",
		},
		{
			name:  "vhd",
			args:  args{"vhd"},
			want:  "disk.vhd",
			want1: "application/x-vhd",
		},
		{
			name:  "vmdk",
			args:  args{"vmdk"},
			want:  "disk.vmdk",
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
			f31 := fedora31.New()
			got, got1, err := f31.FilenameFromType(tt.args.outputFormat)
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
