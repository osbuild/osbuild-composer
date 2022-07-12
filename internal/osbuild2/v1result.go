package osbuild2

// Functions and types for handling old (v1) results.

import (
	"encoding/json"
	"strings"
)

type v1StageResult struct {
	Name     string          `json:"name"`
	Options  json.RawMessage `json:"options"`
	Success  bool            `json:"success"`
	Output   string          `json:"output"`
	Metadata StageMetadata   `json:"metadata"`
}

type v1RawStageResult struct {
	Name     string          `json:"name"`
	Options  json.RawMessage `json:"options"`
	Success  bool            `json:"success"`
	Output   string          `json:"output"`
	Metadata json.RawMessage `json:"metadata"`
}

type v1BuildResult struct {
	Stages  []v1StageResult `json:"stages"`
	TreeID  string          `json:"tree_id"`
	Success bool            `json:"success"`
}

type v1Result struct {
	TreeID    string          `json:"tree_id"`
	OutputID  string          `json:"output_id"`
	Build     *v1BuildResult  `json:"build"`
	Stages    []v1StageResult `json:"stages"`
	Assembler *v1StageResult  `json:"assembler"`
	Success   bool            `json:"success"`
}

func (result *v1StageResult) UnmarshalJSON(data []byte) error {
	var rawStageResult v1RawStageResult
	err := json.Unmarshal(data, &rawStageResult)
	if err != nil {
		return err
	}
	var metadata StageMetadata
	switch {
	case strings.HasSuffix(rawStageResult.Name, "org.osbuild.rpm"):
		metadata = new(RPMStageMetadata)
		err = json.Unmarshal(rawStageResult.Metadata, metadata)
		if err != nil {
			return err
		}
	case strings.HasSuffix(rawStageResult.Name, "org.osbuild.ostree.commit"):
		metadata = new(OSTreeCommitStageMetadata)
		err = json.Unmarshal(rawStageResult.Metadata, metadata)
		if err != nil {
			return err
		}
	default:
		metadata = RawStageMetadata(rawStageResult.Metadata)
	}

	result.Name = rawStageResult.Name
	result.Options = rawStageResult.Options
	result.Success = rawStageResult.Success
	result.Output = rawStageResult.Output
	result.Metadata = metadata

	return nil
}

func (res *Result) fromV1(resv1 v1Result) {
	res.Success = resv1.Success
	res.Type = "result"

	log := make(map[string]PipelineResult)
	metadata := make(map[string]PipelineMetadata)

	// make build pipeline from build result
	if resv1.Build != nil {
		buildResult, buildMetadata := convertStageResults(resv1.Build.Stages)
		log["build"] = buildResult
		if len(buildMetadata) > 0 {
			metadata["build"] = buildMetadata
		}
	}

	// make assembler pipeline from assembler result
	if resv1.Assembler != nil {
		assemblerResult, assemblerMetadata := convertStageResult(resv1.Assembler)
		log["assembler"] = []StageResult{*assemblerResult}
		if assemblerMetadata != nil {
			metadata["assembler"] = map[string]StageMetadata{
				resv1.Assembler.Name: assemblerMetadata,
			}
		}
	}

	// make os pipeline from main stage results
	if len(resv1.Stages) > 0 {
		osResult, osMetadata := convertStageResults(resv1.Stages)
		log["os"] = osResult
		if len(osMetadata) > 0 {
			metadata["os"] = osMetadata
		}
	}

	res.Log = log
	res.Metadata = metadata
}

func convertStageResults(v1Stages []v1StageResult) (PipelineResult, PipelineMetadata) {
	result := make([]StageResult, len(v1Stages))
	metadata := make(map[string]StageMetadata)
	for idx, srv1 := range v1Stages {
		// Implicit memory alasing doesn't couse any bug in this case
		/* #nosec G601 */
		stageResult, stageMetadata := convertStageResult(&srv1)
		result[idx] = *stageResult
		if stageMetadata != nil {
			metadata[stageResult.Type] = stageMetadata
		}
	}
	return result, metadata
}

func convertStageResult(sr1 *v1StageResult) (*StageResult, StageMetadata) {
	sr := &StageResult{
		ID:      "",
		Type:    sr1.Name,
		Output:  sr1.Output,
		Success: sr1.Success,
		Error:   "",
	}

	// the two metadata types we care about (RPM and ostree-commit) share the
	// same structure across v1 and v2 result types, so no conversion is
	// necessary
	return sr, sr1.Metadata
}

// isV1Result returns true if data contains a json-encoded osbuild result
// in version 1 schema.
//
// It detects the schema version by checking if the decoded json contains at
// least one of the three top-level result objects: Build, Stages, or Assembler
//
// error is non-nil when data isn't a json-encoded object.
func isV1Result(data []byte) (bool, error) {
	var v1ResultStub struct {
		Build     interface{} `json:"build"`
		Stages    interface{} `json:"stages"`
		Assembler interface{} `json:"assembler"`
	}

	err := json.Unmarshal(data, &v1ResultStub)
	if err != nil {
		return false, err
	}

	return v1ResultStub.Build != nil || v1ResultStub.Stages != nil || v1ResultStub.Assembler != nil, nil
}
