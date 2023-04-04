package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewZstdStageOptions(t *testing.T) {
	filename := "image.raw.zstd"

	expectedOptions := &ZstdStageOptions{
		Filename: filename,
	}

	actualOptions := NewZstdStageOptions(filename)
	assert.Equal(t, expectedOptions, actualOptions)
}

func TestNewZstdStage(t *testing.T) {
	inputFilename := "image.raw"
	filename := "image.raw.zstd"
	pipeline := "os"

	expectedStage := &Stage{
		Type:    "org.osbuild.zstd",
		Options: NewZstdStageOptions(filename),
		Inputs:  NewZstdStageInputs(NewFilesInputPipelineObjectRef(pipeline, inputFilename, nil)),
	}

	actualStage := NewZstdStage(NewZstdStageOptions(filename),
		NewZstdStageInputs(NewFilesInputPipelineObjectRef(pipeline, inputFilename, nil)))
	assert.Equal(t, expectedStage, actualStage)
}

func TestNewZstdStageNoInputs(t *testing.T) {
	filename := "image.raw.zstd"

	expectedStage := &Stage{
		Type:    "org.osbuild.zstd",
		Options: &ZstdStageOptions{Filename: filename},
		Inputs:  nil,
	}

	actualStage := NewZstdStage(&ZstdStageOptions{Filename: filename}, nil)
	assert.Equal(t, expectedStage, actualStage)
}
