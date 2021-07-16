package osbuild2

// Collection of Inputs for a Stage
type Inputs interface {
	isStageInputs()
}

// Single Input for a Stage
type Input interface {
	isInput()
}

// Fields shared between all Input types (should be embedded in each instance)
type inputCommon struct {
	Type string `json:"type"`
	// Origin should be either 'org.osbuild.source' or 'org.osbuild.pipeline'
	Origin string `json:"origin"`
}

type StageInput interface {
	isStageInput()
}

type References interface {
	isReferences()
}
