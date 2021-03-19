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

// UnmarshalJSON decodes json-encoded StageResult.
//
// This method is here only as a workaround for the default value of the
// success field, see the comment inside the method.
func (sr *StageResult) UnmarshalJSON(data []byte) error {
	// Create a StageResult-like object with the Success value set to true
	// before json.Unmarshal is called. If the success field isn't in the
	// input JSON, the json decoder will not touch it and thus it will still
	// be true.
	//
	// The type alias is needed to prevent recursive calls of this method.
	type stageResultAlias StageResult
	stageResultDefault := stageResultAlias{
		Success: true,
	}

	err := json.Unmarshal(data, &stageResultDefault)
	if err != nil {
		return err
	}

	*sr = StageResult(stageResultDefault)

	return nil
}

type Result struct {
	Type    string                    `json:"type"`
	Success bool                      `json:"success"`
	Error   json.RawMessage           `json:"error"`
	Log     map[string]PipelineResult `json:"log"`
}
