// Package pipeline provides primitives for representing and (un)marshalling
// OSBuild pipelines.
package pipeline

// A Pipeline represents an OSBuild pipeline
type Pipeline struct {
	// BuildPipeline describes how to create the build environment for the
	// following stages and assembler.
	BuildPipeline *Pipeline `json:"build,omitempty"`
	// Sequence of stages that produce the filesystem tree, which is the
	// payload of the produced image.
	Stages []*Stage `json:"stages,omitempty"`
	// Assembler that assembles the filesystem tree into the target image.
	Assembler *Assembler `json:"assembler,omitempty"`
}

// SetBuildPipeline sets the pipeline for generating the build environment for
// a pipeline.
func (p *Pipeline) SetBuildPipeline(buildPipeline *Pipeline) {
	p.BuildPipeline = buildPipeline
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
