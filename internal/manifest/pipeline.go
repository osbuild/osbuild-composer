// Package manifest implements a standard set of osbuild pipelines. A pipeline
// conceptually represents a named filesystem tree, optionally generated
// in a provided build root (represented by another pipeline). All inputs
// to a pipeline must be explicitly specified, either in terms of other
// pipeline, in terms of content addressable inputs or in terms of static
// parameters to the inherited Pipeline structs.
package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type Pipeline interface {
	Name() string
	getBuildPackages() []string
	getPackageSetChain() []rpmmd.PackageSet
	serialize() osbuild2.Pipeline
	getPackageSpecs() []rpmmd.PackageSpec
	getOSTreeCommits() []osTreeCommit
	getInline() []string
}

// A BasePipeline represents the core functionality shared between each of the pipeline
// implementations, and the BasePipeline struct must be embedded in each of them.
type BasePipeline struct {
	name   string
	runner string
	build  *BuildPipeline
}

// Name returns the name of the pipeline. The name must be unique for a given manifest.
// Pipeline names are used to refer to pipelines either as dependencies between pipelines
// or for exporting them.
func (p BasePipeline) Name() string {
	return p.name
}

func (p BasePipeline) getBuildPackages() []string {
	return []string{}
}

func (p BasePipeline) getPackageSetChain() []rpmmd.PackageSet {
	return []rpmmd.PackageSet{}
}

func (p BasePipeline) getPackageSpecs() []rpmmd.PackageSpec {
	return []rpmmd.PackageSpec{}
}

func (p BasePipeline) getOSTreeCommits() []osTreeCommit {
	return []osTreeCommit{}
}

func (p BasePipeline) getInline() []string {
	return []string{}
}

// NewBasePipeline returns a generic Pipeline object. The name is mandatory, immutable and must
// be unique among all the pipelines used in a manifest, which is currently not enforced.
// The build argument is a pipeline representing a build root in which the rest of the
// pipeline is built. In order to ensure reproducibility a build pipeline must always be
// provided, except for int he build pipeline itself. When a build pipeline is not provided
// the build host's filesystem is used as the build root, and in this case a runner must be
// specified which knows how to interpret the host filesystem as a build root.
func NewBasePipeline(name string, build *BuildPipeline, runner *string) BasePipeline {
	p := BasePipeline{
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
func (p BasePipeline) serialize() osbuild2.Pipeline {
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
