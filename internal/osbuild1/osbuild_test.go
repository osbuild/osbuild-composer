// Package osbuild provides primitives for representing and (un)marshalling
// OSBuild types.
package osbuild1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipeline_SetBuild(t *testing.T) {
	expectedPipeline := &Pipeline{
		Build: &Build{
			Pipeline: &Pipeline{},
			Runner:   "org.osbuild.fedora32",
		},
	}
	actualPipeline := &Pipeline{}
	actualPipeline.SetBuild(&Pipeline{}, "org.osbuild.fedora32")
	assert.Equal(t, expectedPipeline, actualPipeline)
}

func TestPipeline_AddStage(t *testing.T) {
	expectedPipeline := &Pipeline{
		Build: &Build{
			Pipeline: &Pipeline{},
			Runner:   "org.osbuild.fedora32",
		},
		Stages: []*Stage{
			{
				Name: "org.osbuild.rpm",
			},
		},
	}
	actualPipeline := &Pipeline{
		Build: &Build{
			Pipeline: &Pipeline{},
			Runner:   "org.osbuild.fedora32",
		},
	}
	actualPipeline.AddStage(&Stage{
		Name: "org.osbuild.rpm",
	})
	assert.Equal(t, expectedPipeline, actualPipeline)
	assert.Equal(t, 1, len(actualPipeline.Stages))
}

func TestPipeline_SetAssembler(t *testing.T) {
	expectedPipeline := &Pipeline{
		Assembler: &Assembler{
			Name: "org.osbuild.testassembler",
		},
	}
	actualPipeline := &Pipeline{}
	actualPipeline.SetAssembler(&Assembler{
		Name: "org.osbuild.testassembler",
	})
	assert.Equal(t, expectedPipeline, actualPipeline)
}
