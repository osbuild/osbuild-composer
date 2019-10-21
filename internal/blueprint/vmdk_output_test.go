package blueprint

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

func Test_vmdkOutput_translate(t *testing.T) {
	type args struct {
		b *Blueprint
	}
	tests := []struct {
		name string
		t    *vmdkOutput
		args args
		want string
	}{
		{
			name: "empty-blueprint",
			t:    &vmdkOutput{},
			args: args{&Blueprint{}},
			want: "pipelines/vmdk_empty_blueprint.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, _ := ioutil.ReadFile(tt.want)
			var want pipeline.Pipeline
			json.Unmarshal([]byte(file), &want)
			if got := tt.t.translate(tt.args.b); !reflect.DeepEqual(got, &want) {
				t.Errorf("vmdkOutput.translate() = %v, want %v", got, &want)
			}
		})
	}
}

func Test_vmdkOutput_getName(t *testing.T) {
	tests := []struct {
		name string
		t    *vmdkOutput
		want string
	}{
		{
			name: "basic",
			t:    &vmdkOutput{},
			want: "image.vmdk",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getName(); got != tt.want {
				t.Errorf("vmdkOutput.getName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_vmdkOutput_getMime(t *testing.T) {
	tests := []struct {
		name string
		t    *vmdkOutput
		want string
	}{
		{
			name: "basic",
			t:    &vmdkOutput{},
			want: "application/x-vmdk",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getMime(); got != tt.want {
				t.Errorf("vmdkOutput.getMime() = %v, want %v", got, tt.want)
			}
		})
	}
}
