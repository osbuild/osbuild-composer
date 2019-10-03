// Package blueprint contains primitives for representing weldr blueprints and
// translating them to OSBuild pipelines
package blueprint

import "osbuild-composer/internal/pipeline"

// A Blueprint is a high-level description of an image.
type Blueprint struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     string    `json:"version,omitempty"`
	Packages    []Package `json:"packages"`
	Modules     []Package `json:"modules"`
	Groups      []Package `json:"groups"`
}

// A Package specifies an RPM package.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// TranslateToPipeline converts the blueprint to a pipeline for a given output format.
func (b *Blueprint) TranslateToPipeline(outputFormat string) *pipeline.Pipeline {
	p := &pipeline.Pipeline{}
	p.SetAssembler(pipeline.NewTarAssembler(pipeline.NewTarAssemblerOptions("image.tar")))
	return p
}
