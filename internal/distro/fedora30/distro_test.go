package fedora30_test

import (
	"github.com/osbuild/osbuild-composer/internal/distro/fedora30"
	"reflect"
	"testing"
)

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
			dist := fedora30.New()
			arch, _ := dist.GetArch("x86_64")
			imgType, err := arch.GetImageType(tt.args.outputFormat)
			if (err != nil) != tt.wantErr {
				t.Errorf("Arch.GetImageType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				got := imgType.Filename()
				got1 := imgType.MIMEType()
				if got != tt.want {
					t.Errorf("ImageType.Filename()  got = %v, want %v", got, tt.want)
				}
				if got1 != tt.want1 {
					t.Errorf("ImageType.MIMEType() got1 = %v, want %v", got1, tt.want1)
				}
			}
		})
	}
}

func TestImageType_BuildPackages(t *testing.T) {
	x8664BuildPackages := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"grub2-pc",
		"policycoreutils",
		"qemu-img",
		"systemd",
		"tar",
		"xz",
	}
	aarch64BuildPackages := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"policycoreutils",
		"qemu-img",
		"systemd",
		"tar",
		"xz",
	}
	buildPackages := map[string][]string{
		"x86_64":  x8664BuildPackages,
		"aarch64": aarch64BuildPackages,
	}
	d := fedora30.New()
	for _, archLabel := range d.ListArchs() {
		archStruct, err := d.GetArch(archLabel)
		if err != nil {
			t.Errorf("d.GetArch(%v) returned err = %v; expected nil", archLabel, err)
			continue
		}
		for _, itLabel := range archStruct.ListImageTypes() {
			itStruct, err := archStruct.GetImageType(itLabel)
			if err != nil {
				t.Errorf("d.GetArch(%v) returned err = %v; expected nil", archLabel, err)
				continue
			}
			reflect.DeepEqual(itStruct.BuildPackages(), buildPackages[archLabel])
		}
	}
}
