package osbuild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFilesInputs(t *testing.T) {
	inputFilename := "image.raw"
	pipeline := "os"

	expectedInput := &FilesInputs{
		File: &FilesInput{
			inputCommon: inputCommon{
				Type:   InputTypeFiles,
				Origin: InputOriginPipeline,
			},
			References: &FilesInputReferencesPipeline{
				fmt.Sprintf("name:%s", pipeline): FileReference{File: inputFilename},
			},
		},
	}

	actualInput := NewFilesInputs(NewFilesInputReferencesPipeline(pipeline, inputFilename))
	assert.Equal(t, expectedInput, actualInput)
}

func TestFilesInput_UnmarshalJSON(t *testing.T) {
	type fields struct {
		Type       string
		Origin     string
		References FilesInputReferences
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
			name: "pipeline-origin",
			fields: fields{
				Type:       InputTypeFiles,
				Origin:     InputOriginPipeline,
				References: NewFilesInputReferencesPipeline("os", "image.raw"),
			},
			args: args{
				data: []byte(`{"type":"org.osbuild.files","origin":"org.osbuild.pipeline","references":{"name:os":{"file":"image.raw"}}}`),
			},
		},
		{
			name: "unknown-origin",
			fields: fields{
				Type:       InputTypeFiles,
				Origin:     InputOriginSource,
				References: nil,
			},
			wantErr: true,
		},
	}

	for idx, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &FilesInput{
				inputCommon: inputCommon{
					Type:   tt.fields.Type,
					Origin: tt.fields.Origin,
				},
				References: tt.fields.References,
			}
			var gotInput FilesInput
			if err := json.Unmarshal(tt.args.data, &gotInput); (err != nil) != tt.wantErr {
				println("data: ", string(tt.args.data))
				t.Errorf("FilesInput.UnmarshalJSON() error = %v, wantErr %v [idx: %d]", err, tt.wantErr, idx)
			}
			if tt.wantErr {
				return
			}
			gotBytes, err := json.Marshal(input)
			if err != nil {
				t.Errorf("Could not marshal FilesInput: %v", err)
			}
			if !bytes.Equal(gotBytes, tt.args.data) {
				t.Errorf("Expected `%v`, got `%v` [idx: %d]", string(tt.args.data), string(gotBytes), idx)
			}
			if !reflect.DeepEqual(&gotInput, input) {
				t.Errorf("got {%v, %v, %v}, expected {%v, %v, %v} [%d]", gotInput.Type, gotInput.Origin, gotInput.References, input.Type, input.Origin, input.References, idx)
			}
		})
	}
}
