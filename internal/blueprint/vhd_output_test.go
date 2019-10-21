package blueprint

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

func Test_vhdOutput_translate(t *testing.T) {
	type args struct {
		b *Blueprint
	}
	tests := []struct {
		name string
		t    *vhdOutput
		args args
		want string
	}{
		{
			name: "empty-blueprint",
			t:    &vhdOutput{},
			args: args{&Blueprint{}},
			want: "pipelines/vhd_empty_blueprint.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, _ := ioutil.ReadFile(tt.want)
			var want pipeline.Pipeline
			json.Unmarshal([]byte(file), &want)
			if got := tt.t.translate(tt.args.b); !reflect.DeepEqual(got, &want) {
				t.Errorf("vhdOutput.translate() = %v, want %v", got, &want)
			}
		})
	}
}

func Test_vhdOutput_getName(t *testing.T) {
	tests := []struct {
		name string
		t    *vhdOutput
		want string
	}{
		{
			name: "basic",
			t:    &vhdOutput{},
			want: "image.vhd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getName(); got != tt.want {
				t.Errorf("vhdOutput.getName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_vhdOutput_getMime(t *testing.T) {
	tests := []struct {
		name string
		t    *vhdOutput
		want string
	}{
		{
			name: "basic",
			t:    &vhdOutput{},
			want: "application/x-vhd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getMime(); got != tt.want {
				t.Errorf("vhdOutput.getMime() = %v, want %v", got, tt.want)
			}
		})
	}
}
