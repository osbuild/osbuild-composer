package osbuild

import (
	"crypto/sha256"
	"fmt"
)

type IgnitionStageInputInline struct {
	InlineFile IgnitionStageInput `json:"inlinefile"`
}

func (IgnitionStageInputInline) isStageInputs() {}

type IgnitionStageInput struct {
	inputCommon
	References IgnitionStageReferences `json:"references"`
}

type IgnitionStageReferences []string

func (IgnitionStageReferences) isReferences() {}

func NewIgnitionInlineInput(embeddedData string) Inputs {
	inputs := new(IgnitionStageInputInline)
	inputs.InlineFile.Type = "org.osbuild.files"
	inputs.InlineFile.Origin = "org.osbuild.source"
	inputs.InlineFile.References = IgnitionStageReferences{fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(embeddedData)))}
	return inputs
}
