// Package osbuild provides primitives for representing and (un)marshalling
// OSBuild types.
package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipeline_AddStage(t *testing.T) {
	expectedPipeline := &Pipeline{
		Build: "name:build",
		Stages: []*Stage{
			{
				Type: "org.osbuild.rpm",
			},
		},
	}
	actualPipeline := &Pipeline{
		Build: "name:build",
	}
	actualPipeline.AddStage(&Stage{
		Type: "org.osbuild.rpm",
	})
	assert.Equal(t, expectedPipeline, actualPipeline)
	assert.Equal(t, 1, len(actualPipeline.Stages))
}
