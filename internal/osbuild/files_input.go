package osbuild

import (
	"encoding/json"
	"fmt"
)

// SPECIFIC INPUT STRUCTURE

type FilesInput struct {
	inputCommon
	References FilesInputRef `json:"references"`
}

const InputTypeFiles string = "org.osbuild.files"

func NewFilesInput(references FilesInputRef) *FilesInput {
	input := new(FilesInput)
	input.Type = InputTypeFiles

	switch t := references.(type) {
	case *FilesInputPipelineArrayRef, *FilesInputPipelineObjectRef:
		input.Origin = InputOriginPipeline
	case *FilesInputSourcePlainRef, *FilesInputSourceArrayRef, *FilesInputSourceObjectRef:
		input.Origin = InputOriginSource
	default:
		panic(fmt.Sprintf("unknown FilesInputReferences type: %v", t))
	}

	input.References = references

	return input
}

type rawFilesInput struct {
	inputCommon
	References json.RawMessage `json:"references"`
}

func (f *FilesInput) UnmarshalJSON(data []byte) error {
	var rawFilesInput rawFilesInput
	if err := json.Unmarshal(data, &rawFilesInput); err != nil {
		return err
	}

	switch rawFilesInput.Origin {
	case InputOriginPipeline:
		possibleRefs := []FilesInputRef{
			&FilesInputPipelineArrayRef{},
			&FilesInputPipelineObjectRef{},
		}
		var err error
		for _, ref := range possibleRefs {
			if err = json.Unmarshal(rawFilesInput.References, ref); err == nil {
				f.References = ref
				break
			}
		}
		if err != nil {
			return fmt.Errorf("FilesInput: failed to unmarshal pipeline references into any supported type")
		}
	case InputOriginSource:
		possibleRefs := []FilesInputRef{
			&FilesInputSourcePlainRef{},
			&FilesInputSourceArrayRef{},
			&FilesInputSourceObjectRef{},
		}
		var err error
		for _, ref := range possibleRefs {
			if err = json.Unmarshal(rawFilesInput.References, ref); err == nil {
				f.References = ref
				break
			}
		}
		if err != nil {
			return fmt.Errorf("FilesInput: failed to unmarshal source references into any supported type")
		}
	default:
		return fmt.Errorf("FilesInput: unknown input origin: %s", rawFilesInput.Origin)
	}

	f.Type = rawFilesInput.Type
	f.Origin = rawFilesInput.Origin

	return nil
}

// SUPPORTED FILE INPUT REFERENCES

// FilesInputRef is an interface that is implemented by all types that can be
// used as a reference in the files input.
type FilesInputRef interface {
	isFilesInputRef()
}

// Type to represent stage-specific metadata that can be passed via the files
// input to the stage.
// The expected JSON structure is:
//
//	`{
//		"<metadata.str1>": <anything_but_object_with_additional_properties>,
//		"<metadata.str2>": <anything_but_object_with_additional_properties>,
//		...
//	}`
type FilesInputRefMetadata interface {
	isFilesInputRefMetadata()
}

// Pipeline Object Reference
// The expected JSON structure is:
//
//	`{
//		"name:<pipeline_name>": {
//			"metadata": {
//				"<metadata.str1>": <anything_but_object_with_additional_properties>,
//				"<metadata.str2>": <anything_but_object_with_additional_properties>,
//				...
//			},
//			"file": "<filename>"
//		},
//		...
//	}`
type FilesInputPipelineObjectRef map[string]FilesInputPipelineOptions

func (*FilesInputPipelineObjectRef) isFilesInputRef() {}

type FilesInputPipelineOptions struct {
	// File to access with in a pipeline
	File string `json:"file,omitempty"`
	// Additional metadata to forward to the stage
	Metadata FilesInputRefMetadata `json:"metadata,omitempty"`
}

func NewFilesInputPipelineObjectRef(pipeline, filename string, metadata FilesInputRefMetadata) FilesInputRef {
	// The files input schema allows for multiple pipelines to be specified, but we don't use it.
	ref := &FilesInputPipelineObjectRef{
		fmt.Sprintf("name:%s", pipeline): {
			File:     filename,
			Metadata: metadata,
		},
	}
	return ref
}

// Pipeline Array Reference
// The expected JSON structure is:
//
//	`[
//		{
//			"id": "name:<pipeline_name>",
//			"options": {
//				"metadata": {
//					"<metadata.str1>": <anything_but_object_with_additional_properties>,
//					"<metadata.str2>": <anything_but_object_with_additional_properties>,
//					...
//				},
//				"file": "<filename>"
//			}
//		},
//		...
//	]`
type FilesInputPipelineArrayRef []FilesInputPipelineArrayRefEntry

func (*FilesInputPipelineArrayRef) isFilesInputRef() {}

type FilesInputPipelineArrayRefEntry struct {
	ID      string                    `json:"id"`
	Options FilesInputPipelineOptions `json:"options,omitempty"`
}

func NewFilesInputPipelineArrayRef(pipeline, filename string, metadata FilesInputRefMetadata) FilesInputRef {
	// The files input schema allows for multiple pipelines to be specified, but we don't use it.
	ref := &FilesInputPipelineArrayRef{
		{
			ID: fmt.Sprintf("name:%s", pipeline),
			Options: FilesInputPipelineOptions{
				File:     filename,
				Metadata: metadata,
			},
		},
	}
	return ref
}

// Source Plain Reference
// The expected JSON structure is:
//
//	`[
//		"sha256:<sha256sum>",
//		...
//	]`
type FilesInputSourcePlainRef []string

func (*FilesInputSourcePlainRef) isFilesInputRef() {}

// NewFilesInputSourcePlainRef creates a FilesInputSourcePlainRef from a list of sha256sums.
// The slice items are the SHA256 checksums of files as a hexadecimal string without any prefix (e.g. "sha256:").
func NewFilesInputSourcePlainRef(sha256Sums []string) FilesInputRef {
	refs := FilesInputSourcePlainRef{}
	for _, sha256Sum := range sha256Sums {
		refs = append(refs, fmt.Sprintf("sha256:%s", sha256Sum))
	}
	return &refs
}

// Source Array Reference
// The expected JSON structure is:
//
//	`[
//		{
//			"id": "sha256:<sha256sum>",
//			"options": {
//				"metadata": {
//					"<metadata.str1>": <anything_but_object_with_additional_properties>,
//					"<metadata.str2>": <anything_but_object_with_additional_properties>,
//					...
//				}
//			}
//		},
//		...
//	]`
type FilesInputSourceArrayRef []FilesInputSourceArrayRefEntry

func (*FilesInputSourceArrayRef) isFilesInputRef() {}

type FilesInputSourceOptions struct {
	// Additional metadata to forward to the stage
	Metadata FilesInputRefMetadata `json:"metadata,omitempty"`
}

type FilesInputSourceArrayRefEntry struct {
	ID      string                   `json:"id"`
	Options *FilesInputSourceOptions `json:"options,omitempty"`
}

// NewFilesInputSourceArrayRefEntry creates a FilesInputSourceArrayRefEntry from a sha256sum and metadata.
// The sha256sum is the SHA256 checksum of the file as a hexadecimal string without any prefix (e.g. "sha256:").
func NewFilesInputSourceArrayRefEntry(sha256Sum string, metadata FilesInputRefMetadata) FilesInputSourceArrayRefEntry {
	ref := FilesInputSourceArrayRefEntry{
		ID: fmt.Sprintf("sha256:%s", sha256Sum),
	}
	if metadata != nil {
		ref.Options = &FilesInputSourceOptions{Metadata: metadata}
	}
	return ref
}

func NewFilesInputSourceArrayRef(entries []FilesInputSourceArrayRefEntry) FilesInputRef {
	ref := FilesInputSourceArrayRef(entries)
	return &ref
}

// Source Object Reference
// The expected JSON structure is:
//
//	`{
//		"sha256:<sha256sum>": {
//			"metadata": {
//				"<metadata.str1>": <anything_but_object_with_additional_properties>,
//				"<metadata.str2>": <anything_but_object_with_additional_properties>,
//				...
//			}
//		},
//		...
//	}`
type FilesInputSourceObjectRef map[string]FilesInputSourceOptions

func (*FilesInputSourceObjectRef) isFilesInputRef() {}

// NewFilesInputSourceObjectRef creates a FilesInputSourceObjectRef from a map of sha256sums to metadata
// The key is the SHA256 checksum of the file as a hexadecimal string without any prefix (e.g. "sha256:").
func NewFilesInputSourceObjectRef(entries map[string]FilesInputRefMetadata) FilesInputRef {
	refs := FilesInputSourceObjectRef{}
	for sha256Sum, metadata := range entries {
		refs[fmt.Sprintf("sha256:%s", sha256Sum)] = FilesInputSourceOptions{Metadata: metadata}
	}
	return &refs
}
