// Package osbuild provides primitives for representing and (un)marshalling
// OSBuild (schema v1) types.
package osbuild1

// A Manifest represents an OSBuild source and pipeline manifest
type Manifest struct {
	Sources  Sources  `json:"sources"`
	Pipeline Pipeline `json:"pipeline"`
}

// A Pipeline represents an OSBuild pipeline
type Pipeline struct {
	// The build environment which can run this pipeline
	Build *Build `json:"build,omitempty"`
	// Sequence of stages that produce the filesystem tree, which is the
	// payload of the produced image.
	Stages []*Stage `json:"stages,omitempty"`
	// Assembler that assembles the filesystem tree into the target image.
	Assembler *Assembler `json:"assembler,omitempty"`
}

type Build struct {
	// Pipeline describes how to create the build root
	Pipeline *Pipeline `json:"pipeline"`
	// The runner to use in this build root
	Runner string `json:"runner"`
}

// SetBuild sets the pipeline and runner for generating the build environment
// for a pipeline.
func (p *Pipeline) SetBuild(pipeline *Pipeline, runner string) {
	p.Build = &Build{
		Pipeline: pipeline,
		Runner:   runner,
	}
}

// AddStage appends a stage to the list of stages of a pipeline. The stages
// will be executed in the order they are appended.
func (p *Pipeline) AddStage(stage *Stage) {
	p.Stages = append(p.Stages, stage)
}

// SetAssembler sets the assembler for a pipeline.
func (p *Pipeline) SetAssembler(assembler *Assembler) {
	p.Assembler = assembler
}
