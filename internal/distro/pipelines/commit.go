package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type OSTreeCommitPipeline struct {
	Pipeline
	treePipeline *OSPipeline
	Ref          string
	OSVersion    string
	Parent       string
}

func NewOSTreeCommitPipeline(buildPipeline *BuildPipeline, treePipeline *OSPipeline) OSTreeCommitPipeline {
	return OSTreeCommitPipeline{
		Pipeline:     New("ostree-commit", &buildPipeline.Pipeline),
		treePipeline: treePipeline,
	}
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
			Ref:       p.Ref,
			OSVersion: p.OSVersion,
			Parent:    p.Parent,
		},
		&osbuild2.OSTreeCommitStageInputs{Tree: commitStageInput}),
	)

	return pipeline
}
