package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewXzStageOptions(t *testing.T) {
	filename := "image.raw.xz"

	expectedOptions := &XzStageOptions{
		Filename: filename,
	}

	actualOptions := NewXzStageOptions(filename)
	assert.Equal(t, expectedOptions, actualOptions)
}

func TestNewXzStage(t *testing.T) {
	inputFilename := "image.raw"
	filename := "image.raw.xz"
	pipeline := "os"

	expectedStage := &Stage{
		Type:    "org.osbuild.xz",
		Options: NewXzStageOptions(filename),
		Inputs:  NewFilesInputs(NewFilesInputReferencesPipeline(pipeline, inputFilename)),
	}

	actualStage := NewXzStage(NewXzStageOptions(filename),
		NewFilesInputs(NewFilesInputReferencesPipeline(pipeline, inputFilename)))
	assert.Equal(t, expectedStage, actualStage)
}

func TestNewXzStageNoInputs(t *testing.T) {
	filename := "image.raw.xz"

	expectedStage := &Stage{
		Type:    "org.osbuild.xz",
		Options: &XzStageOptions{Filename: filename},
		Inputs:  nil,
	}

	actualStage := NewXzStage(&XzStageOptions{Filename: filename}, nil)
	assert.Equal(t, expectedStage, actualStage)
}
