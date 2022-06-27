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
func NewOSTreeCommitPipeline(buildPipeline *BuildPipeline, treePipeline *OSPipeline, ref string) OSTreeCommitPipeline {
	return OSTreeCommitPipeline{
		BasePipeline: NewBasePipeline("ostree-commit", buildPipeline, nil),
		treePipeline: treePipeline,
		ref:          ref,
	}
}

// Ref returns the OSTree ref of the commit.
func (p OSTreeCommitPipeline) Ref() string {
	return p.ref
}

func (p OSTreeCommitPipeline) serialize() osbuild2.Pipeline {
	pipeline := p.BasePipeline.serialize()

	pipeline.AddStage(osbuild2.NewOSTreeInitStage(&osbuild2.OSTreeInitStageOptions{Path: "/repo"}))

	commitStageInput := new(osbuild2.OSTreeCommitStageInput)
	commitStageInput.Type = "org.osbuild.tree"
	commitStageInput.Origin = "org.osbuild.pipeline"
	commitStageInput.References = osbuild2.OSTreeCommitStageReferences{"name:" + p.treePipeline.Name()}

	pipeline.AddStage(osbuild2.NewOSTreeCommitStage(
		&osbuild2.OSTreeCommitStageOptions{
			Ref:       p.Ref(),
			OSVersion: p.OSVersion,
			Parent:    p.treePipeline.OSTreeParent(),
		},
		&osbuild2.OSTreeCommitStageInputs{Tree: commitStageInput}),
	)

	return pipeline
}
