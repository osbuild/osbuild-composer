package osbuild

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestSource_UnmarshalJSON(t *testing.T) {
	type fields struct {
		Name   string
		Source Source
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "invalid json",
			args: args{
				data: []byte(`{"name":"org.osbuild.foo","options":{"bar":null}`),
			},
			wantErr: true,
		},
		{
			name: "unknown source",
			args: args{
				data: []byte(`{"name":"org.osbuild.foo","options":{"bar":null}}`),
			},
			wantErr: true,
		},
		{
			name: "missing options",
			args: args{
				data: []byte(`{"name":"org.osbuild.files"}`),
			},
			wantErr: true,
		},
		{
			name: "missing name",
			args: args{
				data: []byte(`{"foo":null,"options":{"bar":null}}`),
			},
			wantErr: true,
		},
		{
			name: "files-empty",
			fields: fields{
				Name:   "org.osbuild.files",
				Source: &FilesSource{URLs: map[string]FileSource{}},
			},
			args: args{
				data: []byte(`{"org.osbuild.files":{"urls":{}}}`),
			},
		},
		{
			name: "files",
			fields: fields{
				Name: "org.osbuild.files",
				Source: &FilesSource{URLs: map[string]FileSource{
					"checksum1": FileSource{URL: "url1"},
					"checksum2": FileSource{URL: "url2"},
				}},
			},
			args: args{
				data: []byte(`{"org.osbuild.files":{"urls":{"checksum1":{"url":"url1"},"checksum2":{"url":"url2"}}}}`),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := &Sources{
				tt.fields.Name: tt.fields.Source,
			}
			var gotSources Sources
			if err := gotSources.UnmarshalJSON(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("Sources.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			gotBytes, err := json.Marshal(sources)
			if err != nil {
				t.Errorf("Could not marshal source: %v", err)
			}
			if !bytes.Equal(gotBytes, tt.args.data) {
				t.Errorf("Expected '%v', got '%v'", string(tt.args.data), string(gotBytes))
			}
			if !reflect.DeepEqual(&gotSources, sources) {
				t.Errorf("got '%v', expected '%v'", &gotSources, sources)
			}
		})
	}
}
