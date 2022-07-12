package osbuild2

// Functions and types for handling old (v1) results.

import (
	"encoding/json"

	"github.com/osbuild/osbuild-composer/internal/osbuild1"
)

func (res *Result) fromV1(resv1 osbuild1.Result) {
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

func convertStageResults(v1Stages []osbuild1.StageResult) (PipelineResult, PipelineMetadata) {
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

func convertStageResult(sr1 *osbuild1.StageResult) (*StageResult, StageMetadata) {
	sr := &StageResult{
		ID:      "",
		Type:    sr1.Name,
		Output:  sr1.Output,
		Success: sr1.Success,
		Error:   "",
	}

	var md StageMetadata = nil
	if sr1.Metadata != nil {
		switch md1 := sr1.Metadata.(type) {
		case *osbuild1.RPMStageMetadata:
			rpmmd := new(RPMStageMetadata)
			rpmmd.Packages = make([]RPMPackageMetadata, len(md1.Packages))
			for idx, pkg := range md1.Packages {
				rpmmd.Packages[idx] = RPMPackageMetadata(pkg)
			}
			md = rpmmd
		case *osbuild1.OSTreeCommitStageMetadata:
			commitmd := new(OSTreeCommitStageMetadata)
			commitmd.Compose = OSTreeCommitStageMetadataCompose(md1.Compose)
			md = commitmd
		}

	}
	return sr, md
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
