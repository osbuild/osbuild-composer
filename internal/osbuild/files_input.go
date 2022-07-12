package osbuild

import (
	"encoding/json"
	"fmt"
)

// Inputs for individual files

type FilesInputs struct {
	File *FilesInput `json:"file"`
}

func (FilesInputs) isStageInputs() {}

func NewFilesInputs(references FilesInputReferences) *FilesInputs {
	return &FilesInputs{
		File: NewFilesInput(references),
	}
}

// IMPLEMENTED INTERFACES OF STAGES ACCEPTING THIS INPUTS TYPE

// inputs accepted by the XZ stage
func (FilesInputs) isXzStageInputs() {}

// inputs accepted by the Copy stage
func (FilesInputs) isCopyStageInputs() {}

// SPECIFIC INPUT STRUCTURE

type FilesInput struct {
	inputCommon
	References FilesInputReferences `json:"references"`
}

const InputTypeFiles string = "org.osbuild.files"

func NewFilesInput(references FilesInputReferences) *FilesInput {
	input := new(FilesInput)
	input.Type = InputTypeFiles

	switch t := references.(type) {
	case *FilesInputReferencesPipeline:
		input.Origin = InputOriginPipeline
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

	var ref FilesInputReferences
	switch rawFilesInput.Origin {
	case InputOriginPipeline:
		ref = &FilesInputReferencesPipeline{}
	default:
		return fmt.Errorf("FilesInput: unknown input origin: %s", rawFilesInput.Origin)
	}

	if err := json.Unmarshal(rawFilesInput.References, ref); err != nil {
		return err
	}

	f.Type = rawFilesInput.Type
	f.Origin = rawFilesInput.Origin
	f.References = ref

	return nil
}

// SUPPORTED FILE INPUT REFERENCES

type FilesInputReferences interface {
	isFilesInputReferences()
}

// The expected JSON structure is:
// `"name:<pipeline_name>": {"file": "<filename>"}`
type FilesInputReferencesPipeline map[string]FileReference

func (*FilesInputReferencesPipeline) isFilesInputReferences() {}

type FileReference struct {
	File string `json:"file"`
}

func NewFilesInputReferencesPipeline(pipeline, filename string) FilesInputReferences {
	ref := &FilesInputReferencesPipeline{
		fmt.Sprintf("name:%s", pipeline): {File: filename},
	}
	return ref
}

// TODO: define FilesInputReferences for "sources"
