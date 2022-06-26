package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type OSTreeCommitPipeline struct {
	Pipeline
	treePipeline *OSPipeline
	OSVersion    string

	ref string
}

func NewOSTreeCommitPipeline(buildPipeline *BuildPipeline, treePipeline *OSPipeline, ref string) OSTreeCommitPipeline {
	return OSTreeCommitPipeline{
		Pipeline:     New("ostree-commit", buildPipeline, nil),
		treePipeline: treePipeline,
		ref:          ref,
	}
}

func (p OSTreeCommitPipeline) Ref() string {
	return p.ref
}

func (p OSTreeCommitPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

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
