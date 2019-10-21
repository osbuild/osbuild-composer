package blueprint

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

func Test_ext4Output_translate(t *testing.T) {
	type args struct {
		b *Blueprint
	}
	tests := []struct {
		name string
		t    *ext4Output
		args args
		want string
	}{
		{
			name: "empty-blueprint",
			t:    &ext4Output{},
			args: args{&Blueprint{}},
			want: "pipelines/ext4_empty_blueprint.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, _ := ioutil.ReadFile(tt.want)
			var want pipeline.Pipeline
			json.Unmarshal([]byte(file), &want)
			if got := tt.t.translate(tt.args.b); !reflect.DeepEqual(got, &want) {
				t.Errorf("ext4Output.translate() = %v, want %v", got, &want)
			}
		})
	}
}

func Test_ext4Output_getName(t *testing.T) {
	tests := []struct {
		name string
		t    *ext4Output
		want string
	}{
		{
			name: "basic",
			t:    &ext4Output{},
			want: "image.img",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getName(); got != tt.want {
				t.Errorf("ext4Output.getName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ext4Output_getMime(t *testing.T) {
	tests := []struct {
		name string
		t    *ext4Output
		want string
	}{
		{
			name: "basic",
			t:    &ext4Output{},
			want: "application/octet-stream",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getMime(); got != tt.want {
				t.Errorf("ext4Output.getMime() = %v, want %v", got, tt.want)
			}
		})
	}
}
