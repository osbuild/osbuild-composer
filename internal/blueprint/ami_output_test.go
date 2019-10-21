package blueprint

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

func Test_amiOutput_translate(t *testing.T) {
	type args struct {
		b *Blueprint
	}
	tests := []struct {
		name string
		t    *amiOutput
		args args
		want string
	}{
		{
			name: "empty-blueprint",
			t:    &amiOutput{},
			args: args{&Blueprint{}},
			want: "pipelines/ami_empty_blueprint.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, _ := ioutil.ReadFile(tt.want)
			var want pipeline.Pipeline
			json.Unmarshal([]byte(file), &want)
			if got := tt.t.translate(tt.args.b); !reflect.DeepEqual(got, &want) {
				t.Errorf("amiOutput.translate() = %v, want %v", got, &want)
			}
		})
	}
}

func Test_amiOutput_getName(t *testing.T) {
	tests := []struct {
		name string
		t    *amiOutput
		want string
	}{
		{
			name: "basic",
			t:    &amiOutput{},
			want: "image.ami",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getName(); got != tt.want {
				t.Errorf("amiOutput.getName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_amiOutput_getMime(t *testing.T) {
	tests := []struct {
		name string
		t    *amiOutput
		want string
	}{
		{
			name: "basic",
			t:    &amiOutput{},
			want: "application/x-qemu-disk",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getMime(); got != tt.want {
				t.Errorf("amiOutput.getMime() = %v, want %v", got, tt.want)
			}
		})
	}
}
