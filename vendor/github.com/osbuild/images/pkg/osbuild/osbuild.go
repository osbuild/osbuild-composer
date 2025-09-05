// Package osbuild provides primitives for representing and (un)marshalling
// OSBuild (schema v2) types.
package osbuild

import (
	"encoding/json"
	"fmt"
)

const (
	// should be "^\\/(?!\\.\\.)((?!\\/\\.\\.\\/).)+$" but Go doesn't support lookaheads
	// therefore we have to instead check for the invalid cases, which is much simpler
	invalidPathRegex = `((^|\/)[.]{2}(\/|$))|^([^/].*)*$`
)

// A Manifest represents an OSBuild source and pipeline manifest
type Manifest struct {
	Version   string     `json:"version"`
	Pipelines []Pipeline `json:"pipelines"`
	Sources   Sources    `json:"sources"`
}

// A Pipeline represents an OSBuild pipeline
type Pipeline struct {
	Name string `json:"name,omitempty"`
	// The build environment which can run this pipeline
	Build string `json:"build,omitempty"`

	Runner string `json:"runner,omitempty"`

	// Sequence of stages that produce the filesystem tree, which is the
	// payload of the produced image.
	Stages []*Stage `json:"stages,omitempty"`
}

// SetBuild sets the pipeline and runner for generating the build environment
// for a pipeline.
func (p *Pipeline) SetBuild(build string) {
	p.Build = build
}

// AddStage appends a stage to the list of stages of a pipeline. The stages
// will be executed in the order they are appended.
// If the argument is nil, it is not added.
func (p *Pipeline) AddStage(stage *Stage) {
	if stage != nil {
		p.Stages = append(p.Stages, stage)
	}
}

// AddStages appends multiple stages to the list of stages of a pipeline. The
// stages will be executed in the order they are appended.
// If the argument is nil, it is not added.
func (p *Pipeline) AddStages(stages ...*Stage) {
	for _, stage := range stages {
		p.AddStage(stage)
	}
}

// Take some bytes and deserialize them into a Manifest; mostly used to take
// an inspected manifest
func NewManifestFromBytes(data []byte) (*Manifest, error) {
	manifest := &Manifest{}

	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return manifest, nil
}

// GetID gets the pipeline identifiers for an *inspected* manifest. These are
// not available for non-inspected manifests and will return an error there.
func (p *Pipeline) GetID() (string, error) {
	if len(p.Stages) == 0 {
		return "", fmt.Errorf("no stages in manifest")
	}

	lastStage := p.Stages[len(p.Stages)-1]

	if len(lastStage.ID) == 0 {
		return "", fmt.Errorf("un-inspected manifest, identifiers are not available")
	}

	return lastStage.ID, nil
}
