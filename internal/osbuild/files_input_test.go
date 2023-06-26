package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilesInput_UnmarshalJSON(t *testing.T) {
	testCases := []struct {
		name    string
		ref     FilesInputRef
		rawJson []byte
	}{
		{
			name:    "pipeline-object-ref",
			ref:     NewFilesInputPipelineObjectRef("os", "image.raw", nil),
			rawJson: []byte(`{"type":"org.osbuild.files","origin":"org.osbuild.pipeline","references":{"name:os":{"file":"image.raw"}}}`),
		},
		{
			name:    "pipeline-array-ref",
			ref:     NewFilesInputPipelineArrayRef("os", "image.raw", nil),
			rawJson: []byte(`{"type":"org.osbuild.files","origin":"org.osbuild.pipeline","references":[{"id":"name:os","options":{"file":"image.raw"}}]}`),
		},
		{
			name:    "source-plain-ref",
			ref:     NewFilesInputSourcePlainRef([]string{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"}),
			rawJson: []byte(`{"type":"org.osbuild.files","origin":"org.osbuild.source","references":["sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"]}`),
		},
		{
			name: "source-array-ref",
			ref: NewFilesInputSourceArrayRef([]FilesInputSourceArrayRefEntry{
				NewFilesInputSourceArrayRefEntry("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", nil),
			}),
			rawJson: []byte(`{"type":"org.osbuild.files","origin":"org.osbuild.source","references":[{"id":"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"}]}`),
		},
		{
			name: "source-object-ref",
			ref: NewFilesInputSourceObjectRef(map[string]FilesInputRefMetadata{
				"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef": nil,
			}),
			rawJson: []byte(`{"type":"org.osbuild.files","origin":"org.osbuild.source","references":{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef":{}}}`),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var gotInput FilesInput
			err := json.Unmarshal(tt.rawJson, &gotInput)
			assert.NoErrorf(t, err, "FilesInput.UnmarshalJSON() error = %v", err)

			input := NewFilesInput(tt.ref)
			gotBytes, err := json.Marshal(input)
			assert.NoErrorf(t, err, "FilesInput.MarshalJSON() error = %v", err)

			assert.EqualValuesf(t, tt.rawJson, gotBytes, "Expected JSON `%v`, got JSON `%v`", string(tt.rawJson), string(gotBytes))
			assert.EqualValuesf(t, input, &gotInput, "Expected input `%v`, got input `%v` [test: %q]", input, &gotInput, tt.name)
		})
	}

	// test invalid cases
	invalidTestCases := []struct {
		name    string
		rawJson []byte
	}{
		{
			name:    "invalid-pipeline-ref",
			rawJson: []byte(`{"type":"org.osbuild.files","origin":"org.osbuild.pipeline","references":1}`),
		},
		{
			name:    "invalid-source-ref",
			rawJson: []byte(`{"type":"org.osbuild.files","origin":"org.osbuild.source","references":2}`),
		},
		{
			name:    "invalid-origin",
			rawJson: []byte(`{"type":"org.osbuild.files","origin":"org.osbuild.invalid","references":{}}`),
		},
		{
			name:    "invalid-input",
			rawJson: []byte(`[]`),
		},
	}

	for _, tt := range invalidTestCases {
		t.Run(tt.name, func(t *testing.T) {
			var gotInput FilesInput
			err := json.Unmarshal(tt.rawJson, &gotInput)
			assert.Error(t, err)
		})
	}

}
