package blueprint

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

func Test_tarOutput_translate(t *testing.T) {
	type args struct {
		b *Blueprint
	}
	tests := []struct {
		name string
		t    *tarOutput
		args args
		want string
	}{
		{
			name: "empty-blueprint",
			t:    &tarOutput{},
			args: args{&Blueprint{}},
			want: "pipelines/tar_empty_blueprint.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, _ := ioutil.ReadFile(tt.want)
			var want pipeline.Pipeline
			json.Unmarshal([]byte(file), &want)
			if got := tt.t.translate(tt.args.b); !reflect.DeepEqual(got, &want) {
				t.Errorf("tarOutput.translate() = %v, want %v", got, &want)
			}
		})
	}
}

func Test_tarOutput_getName(t *testing.T) {
	tests := []struct {
		name string
		t    *tarOutput
		want string
	}{
		{
			name: "basic",
			t:    &tarOutput{},
			want: "image.tar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getName(); got != tt.want {
				t.Errorf("tarOutput.getName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_tarOutput_getMime(t *testing.T) {
	tests := []struct {
		name string
		t    *tarOutput
		want string
	}{
		{
			name: "basic",
			t:    &tarOutput{},
			want: "application/x-tar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getMime(); got != tt.want {
				t.Errorf("tarOutput.getMime() = %v, want %v", got, tt.want)
			}
		})
	}
}
