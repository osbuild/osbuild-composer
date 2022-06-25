package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type Pipeline struct {
	name   string
	runner *string
	build  *Pipeline
}

func (p Pipeline) Name() string {
	return p.name
}

func New(name string, build *Pipeline) Pipeline {
	return Pipeline{
		name:  name,
		build: build,
	}
}

func (p Pipeline) Serialize() osbuild2.Pipeline {
	var buildName string
	if p.build != nil {
		buildName = "name:" + p.build.Name()
	}
	var runner string
	if p.runner != nil {
		runner = *p.runner
	}
	return osbuild2.Pipeline{
		Name:   p.name,
		Runner: runner,
		Build:  buildName,
	}
}
