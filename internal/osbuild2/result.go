package osbuild2

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/osbuild/osbuild-composer/internal/osbuild1"
)

type Result struct {
	Type     string                      `json:"type"`
	Success  bool                        `json:"success"`
	Error    json.RawMessage             `json:"error"`
	Log      map[string]PipelineResult   `json:"log"`
	Metadata map[string]PipelineMetadata `json:"metadata"`
}

type PipelineResult []StageResult

type StageResult struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Output  string `json:"output"`
	Success bool   `json:"success,omitempty"`
	Error   string `json:"string,omitempty"`
}

type PipelineMetadata map[string]StageMetadata

type StageMetadata interface {
	isStageMetadata()
}

// RawStageMetadata is used to store the metadata from a stage that doesn't
// define its own structure
type RawStageMetadata json.RawMessage

func (RawStageMetadata) isStageMetadata() {}

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

func (md PipelineMetadata) UnmarshalJSON(data []byte) error {
	var rawPipelineMetadata map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawPipelineMetadata); err != nil {
		return err
	}
	pmd := make(map[string]StageMetadata)
	var metadata StageMetadata
	for name, rawStageData := range rawPipelineMetadata {
		switch name {
		case "org.osbuild.rpm":
			metadata = new(RPMStageMetadata)
			if err := json.Unmarshal(rawStageData, metadata); err != nil {
				return err
			}
		case "org.osbuild.ostree.commit":
			metadata = new(OSTreeCommitStageMetadata)
			if err := json.Unmarshal(rawStageData, metadata); err != nil {
				return err
			}
		default:
			metadata = RawStageMetadata(rawStageData)
		}
		pmd[name] = metadata
	}
	md = pmd
	return nil
}

func (res *Result) UnmarshalJSON(data []byte) error {
	// detect if the input is v1 result
	if v1Result, err := isV1Result(data); err != nil {
		return err
	} else if v1Result {
		var resv1 osbuild1.Result
		if err := json.Unmarshal(data, &resv1); err != nil {
			return err
		}
		res.fromV1(resv1)
		return nil
	}

	// otherwise, unmarshal using a type alias to prevent recursive calls to
	// this method
	type resultAlias Result
	var resv2 resultAlias
	if err := json.Unmarshal(data, &resv2); err != nil {
		return err
	}

	*res = Result(resv2)
	return nil
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

func convertStageResults(v1Stages []osbuild1.StageResult) (PipelineResult, PipelineMetadata) {
	result := make([]StageResult, len(v1Stages))
	metadata := make(map[string]StageMetadata)
	for idx, srv1 := range v1Stages {
		stageResult, stageMetadata := convertStageResult(&srv1)
		result[idx] = *stageResult
		if stageMetadata != nil {
			metadata[stageResult.Type] = stageMetadata
		}
	}
	return result, metadata
}

func (res *Result) fromV1(resv1 osbuild1.Result) {
	res.Success = resv1.Success
	res.Type = "result"

	log := make(map[string]PipelineResult)
	metadata := make(map[string]PipelineMetadata)

	// make build pipeline from build result
	buildResult, buildMetadata := convertStageResults(resv1.Build.Stages)
	log["build"] = buildResult
	if len(buildMetadata) > 0 {
		metadata["build"] = buildMetadata
	}

	// make assembler pipeline from assembler result
	assemblerResult, assemblerMetadata := convertStageResult(resv1.Assembler)
	log["assembler"] = []StageResult{*assemblerResult}
	if assemblerMetadata != nil {
		metadata["assembler"] = map[string]StageMetadata{
			resv1.Assembler.Name: assemblerMetadata,
		}
	}

	// make os pipeline from main stage results
	osResult, osMetadata := convertStageResults(resv1.Stages)
	log["os"] = osResult
	if len(buildMetadata) > 0 {
		metadata["os"] = osMetadata
	}

	res.Log = log
	res.Metadata = metadata
}

func (res *Result) Write(writer io.Writer) error {
	if res.Log == nil {
		fmt.Fprintf(writer, "The compose result is empty.\n")
	}

	// The pipeline results don't have a stable order
	// (see https://github.com/golang/go/issues/27179)
	// Sort based on pipeline name to have a stable print order
	pipelineNames := make([]string, 0, len(res.Log))
	for name := range res.Log {
		pipelineNames = append(pipelineNames, name)
	}
	sort.Strings(pipelineNames)

	for _, pipelineName := range pipelineNames {
		fmt.Fprintf(writer, "Pipeline %s\n", pipelineName)
		pipelineMD := res.Metadata[pipelineName]
		for _, stage := range res.Log[pipelineName] {
			fmt.Fprintf(writer, "Stage %s\n", stage.Type)
			fmt.Fprintf(writer, "Output:\n%s\n", stage.Output)

			// print structured stage metadata if available
			if pipelineMD != nil {
				if md, ok := pipelineMD[stage.Type]; ok {
					fmt.Fprint(writer, "Metadata:\n")
					enc := json.NewEncoder(writer)
					enc.SetIndent("", "  ")
					err := enc.Encode(md)
					if err != nil {
						return err
					}
				}
				fmt.Fprint(writer, "\n")
			}
		}
	}

	return nil
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
