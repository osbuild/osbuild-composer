package blueprint

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

func Test_liveIsoOutput_translate(t *testing.T) {
	type args struct {
		b *Blueprint
	}
	tests := []struct {
		name string
		t    *liveIsoOutput
		args args
		want string
	}{
		{
			name: "empty-blueprint",
			t:    &liveIsoOutput{},
			args: args{&Blueprint{}},
			want: "pipelines/liveiso_empty_blueprint.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, _ := ioutil.ReadFile(tt.want)
			var want pipeline.Pipeline
			json.Unmarshal([]byte(file), &want)
			if got := tt.t.translate(tt.args.b); !reflect.DeepEqual(got, &want) {
				t.Errorf("liveIsoOutput.translate() = %v, want %v", got, &want)
			}
		})
	}
}

func Test_liveIsoOutput_getName(t *testing.T) {
	tests := []struct {
		name string
		t    *liveIsoOutput
		want string
	}{
		{
			name: "basic",
			t:    &liveIsoOutput{},
			want: "image.iso",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getName(); got != tt.want {
				t.Errorf("liveIsoOutput.getName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_liveIsoOutput_getMime(t *testing.T) {
	tests := []struct {
		name string
		t    *liveIsoOutput
		want string
	}{
		{
			name: "basic",
			t:    &liveIsoOutput{},
			want: "application/x-iso9660-image",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getMime(); got != tt.want {
				t.Errorf("liveIsoOutput.getMime() = %v, want %v", got, tt.want)
			}
		})
	}
}
