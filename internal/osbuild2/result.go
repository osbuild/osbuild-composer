package osbuild2

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

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

func (md *PipelineMetadata) UnmarshalJSON(data []byte) error {
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
	*md = pmd
	return nil
}

type Result struct {
	Type     string                      `json:"type"`
	Success  bool                        `json:"success"`
	Error    json.RawMessage             `json:"error"`
	Log      map[string]PipelineResult   `json:"log"`
	Metadata map[string]PipelineMetadata `json:"metadata"`
}

func (cr *Result) Write(writer io.Writer) error {
	if cr.Log == nil {
		fmt.Fprintf(writer, "The compose result is empty.\n")
	}

	// The pipeline results don't have a stable order
	// (see https://github.com/golang/go/issues/27179)
	// Sort based on pipeline name to have a stable print order
	pipelineNames := make([]string, 0, len(cr.Log))
	for name := range cr.Log {
		pipelineNames = append(pipelineNames, name)
	}
	sort.Strings(pipelineNames)

	for _, pipelineName := range pipelineNames {
		fmt.Fprintf(writer, "Pipeline %s\n", pipelineName)
		pipelineMD := cr.Metadata[pipelineName]
		for _, stage := range cr.Log[pipelineName] {
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
