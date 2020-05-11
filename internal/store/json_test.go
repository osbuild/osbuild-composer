package store

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
)

func Test_imageTypeToCompatString(t *testing.T) {
	type args struct {
		input distro.ImageType
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "valid",
			args: args{
				input: &test_distro.TestImageType{},
			},
			want: "test_type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imageTypeToCompatString(tt.args.input)
			if got != tt.want {
				t.Errorf("imageTypeStringToCompatString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_imageTypeFromCompatString(t *testing.T) {
	type args struct {
		input string
		arch  distro.Arch
	}
	tests := []struct {
		name string
		args args
		want distro.ImageType
	}{
		{
			name: "valid",
			args: args{
				input: "test_type",
				arch:  &test_distro.TestArch{},
			},
			want: &test_distro.TestImageType{},
		},
		{
			name: "invalid",
			args: args{
				input: "foo",
				arch:  &test_distro.TestArch{},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imageTypeFromCompatString(tt.args.input, tt.args.arch)
			if got != tt.want {
				t.Errorf("imageTypeStringFromCompatString() got = %v, want %v", got, tt.want)
			}
		})
	}
}
