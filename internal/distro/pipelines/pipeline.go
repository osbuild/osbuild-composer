// Package pipeline implements a standard set of osbuild pipelines. A pipeline
// conceptually represents a named filesystem tree, optionally generated
// in a provided build root (represented by another pipeline). All inputs
// to a pipeline must be explicitly specified, either in terms of other
// pipeline, in terms of content addressable inputs or in terms of static
// parameters to the inherited Pipeline structs.
package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type BootLoader uint64

const (
	BOOTLOADER_GRUB BootLoader = iota
	BOOTLOADER_ZIPL
)

// A Pipeline represents the core functionality shared between each of the pipeline
// implementations, and the Pipeline struct must be embedded in each of them.
type Pipeline struct {
	name   string
	runner string
	build  *BuildPipeline
}

// Name returns the name of the pipeline. The name must be unique for a given manifest.
// Pipeline names are used to refer to pipelines either as dependencies between pipelines
// or for exporting them.
func (p Pipeline) Name() string {
	return p.name
}

// New returns a generic Pipeline object. The name is mandatory, immutable and must
// be unique among all the pipelines used in a manifest, which is currently not enforced.
// The build argument is a pipeline representing a build root in which the rest of the
// pipeline is built. In order to ensure reproducibility a build pipeline must always be
// provided, except for int he build pipeline itself. When a build pipeline is not provided
// the build host's filesystem is used as the build root, and in this case a runner must be
// specified which knows how to interpret the host filesystem as a build root.
func New(name string, build *BuildPipeline, runner *string) Pipeline {
	p := Pipeline{
		name:  name,
		build: build,
	}
	if runner != nil {
		if build != nil {
			panic("both runner and build pipeline specified")
		}
		p.runner = *runner
	} else if build == nil {
		panic("neither build pipeline nor runner specified")
	}
	return p
}

// Serialize turns a given pipeline into an osbuild2.Pipeline object. This object is
// meant to be treated as opaque and not to be modified further outside of the pipeline
// package.
func (p Pipeline) Serialize() osbuild2.Pipeline {
	var buildName string
	if p.build != nil {
		buildName = "name:" + p.build.Name()
	}
	return osbuild2.Pipeline{
		Name:   p.name,
		Runner: p.runner,
		Build:  buildName,
	}
}
