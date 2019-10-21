package blueprint

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

func Test_openstackOutput_translate(t *testing.T) {
	type args struct {
		b *Blueprint
	}
	tests := []struct {
		name string
		t    *openstackOutput
		args args
		want string
	}{
		{
			name: "empty-blueprint",
			t:    &openstackOutput{},
			args: args{&Blueprint{}},
			want: "pipelines/openstack_empty_blueprint.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, _ := ioutil.ReadFile(tt.want)
			var want pipeline.Pipeline
			json.Unmarshal([]byte(file), &want)
			if got := tt.t.translate(tt.args.b); !reflect.DeepEqual(got, &want) {
				t.Errorf("openstackOutput.translate() = %v, want %v", got, &want)
			}
		})
	}
}

func Test_openstackOutput_getName(t *testing.T) {
	tests := []struct {
		name string
		t    *openstackOutput
		want string
	}{
		{
			name: "basic",
			t:    &openstackOutput{},
			want: "image.qcow2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getName(); got != tt.want {
				t.Errorf("openstackOutput.getName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_openstackOutput_getMime(t *testing.T) {
	tests := []struct {
		name string
		t    *openstackOutput
		want string
	}{
		{
			name: "basic",
			t:    &openstackOutput{},
			want: "application/x-qemu-disk",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.getMime(); got != tt.want {
				t.Errorf("openstackOutput.getMime() = %v, want %v", got, tt.want)
			}
		})
	}
}
