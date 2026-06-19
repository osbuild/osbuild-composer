package osbuild

import (
	"crypto/sha256"
	"fmt"
)

type IgnitionStageOptions struct {
	Network []string `json:"network,omitempty"`
	Target  string   `json:"target,omitempty"`
}

func (IgnitionStageOptions) isStageOptions() {}

func NewIgnitionStage(options *IgnitionStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.ignition",
		Options: options,
	}
}

func NewEmptyIgnitionStage() *Stage {
	return &Stage{
		Type: "org.osbuild.ignition",
	}
}

type IgnitionStageInputInline struct {
	InlineFile *FilesInput `json:"inlinefile"`
}

func (IgnitionStageInputInline) isStageInputs() {}

func NewIgnitionInlineInput(embeddedData string) Inputs {
	input := NewFilesInput(NewFilesInputSourcePlainRef([]string{
		fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(embeddedData))),
	}))
	return &IgnitionStageInputInline{InlineFile: input}
}
