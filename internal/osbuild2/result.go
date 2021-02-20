package osbuild2

import (
	"encoding/json"
)

type PipelineResult []StageResult

type StageResult struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Output  string `json:"output"`
	Success bool   `json:"success,omitempty"`
	Error   string `json:"string,omitempty"`
}

type Result struct {
	Type    string                    `json:"type"`
	Success bool                      `json:"success"`
	Error   json.RawMessage           `json:"error"`
	Log     map[string]PipelineResult `json:"log"`
}
