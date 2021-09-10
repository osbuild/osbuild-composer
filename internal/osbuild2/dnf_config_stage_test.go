package osbuild2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDNFConfigStageOptions(t *testing.T) {
	variables := []DNFVariable{
		{
			Name:  "release",
			Value: "8.4",
		},
	}

	expectedOptions := &DNFConfigStageOptions{
		Variables: variables,
	}
	actualOptions := NewDNFConfigStageOptions(variables)
	assert.Equal(t, expectedOptions, actualOptions)
}

func TestNewDNFConfigStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.dnf.config",
		Options: &DNFConfigStageOptions{},
	}
	actualStage := NewDNFConfigStage(&DNFConfigStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}
