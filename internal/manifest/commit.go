package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild"
)

// OSTreeCommit represents an ostree with one commit.
type OSTreeCommit struct {
	Base
	OSVersion string

	treePipeline *OS
	ref          string
}

// NewOSTreeCommit creates a new OSTree commit pipeline. The
// treePipeline is the tree representing the content of the commit.
// ref is the ref to create the commit under.
func NewOSTreeCommit(m *Manifest,
	buildPipeline *Build,
	treePipeline *OS,
	ref string) *OSTreeCommit {
	p := &OSTreeCommit{
		Base:         NewBase(m, "ostree-commit", buildPipeline),
		treePipeline: treePipeline,
		ref:          ref,
	}
	if treePipeline.Base.manifest != m {
		panic("tree pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *OSTreeCommit) getBuildPackages() []string {
	packages := []string{
		"rpm-ostree",
	}
	return packages
}

func (p *OSTreeCommit) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	if p.treePipeline.OSTreeRef == "" {
		panic("tree is not ostree")
	}

	pipeline.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: "/repo"}))

	var parent string
	if p.treePipeline.OSTreeParent != nil {
		parent = p.treePipeline.OSTreeParent.Checksum
	}
	pipeline.AddStage(osbuild.NewOSTreeCommitStage(
		&osbuild.OSTreeCommitStageOptions{
			Ref:       p.ref,
			OSVersion: p.OSVersion,
			Parent:    parent,
		},
		p.treePipeline.Name()),
	)

	return pipeline
}
