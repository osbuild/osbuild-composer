package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// OSTreeCommitPipeline represents an ostree with one commit.
type OSTreeCommitPipeline struct {
	BasePipeline
	OSVersion string

	treePipeline *OSPipeline
	ref          string
}

// NewOSTreeCommitPipeline creates a new OSTree commit pipeline. The
// treePipeline is the tree representing the content of the commit.
// ref is the ref to create the commit under.
func NewOSTreeCommitPipeline(m *Manifest,
	buildPipeline *BuildPipeline,
	treePipeline *OSPipeline,
	ref string) *OSTreeCommitPipeline {
	p := &OSTreeCommitPipeline{
		BasePipeline: NewBasePipeline(m, "ostree-commit", buildPipeline, nil),
		treePipeline: treePipeline,
		ref:          ref,
	}
	if treePipeline.BasePipeline.manifest != m {
		panic("tree pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *OSTreeCommitPipeline) getBuildPackages() []string {
	packages := []string{
		"rpm-ostree",
	}
	return packages
}

func (p *OSTreeCommitPipeline) serialize() osbuild2.Pipeline {
	pipeline := p.BasePipeline.serialize()

	if p.treePipeline.OSTree == nil {
		panic("tree is not ostree")
	}

	pipeline.AddStage(osbuild2.NewOSTreeInitStage(&osbuild2.OSTreeInitStageOptions{Path: "/repo"}))

	commitStageInput := new(osbuild2.OSTreeCommitStageInput)
	commitStageInput.Type = "org.osbuild.tree"
	commitStageInput.Origin = "org.osbuild.pipeline"
	commitStageInput.References = osbuild2.OSTreeCommitStageReferences{"name:" + p.treePipeline.Name()}

	var parent string
	if p.treePipeline.OSTree.Parent != nil {
		parent = p.treePipeline.OSTree.Parent.Checksum
	}
	pipeline.AddStage(osbuild2.NewOSTreeCommitStage(
		&osbuild2.OSTreeCommitStageOptions{
			Ref:       p.ref,
			OSVersion: p.OSVersion,
			Parent:    parent,
		},
		&osbuild2.OSTreeCommitStageInputs{Tree: commitStageInput}),
	)

	return pipeline
}
