package osbuild1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type rawAssemblerResult struct {
	Name    string          `json:"name"`
	Options json.RawMessage `json:"options"`
	Success bool            `json:"success"`
	Output  string          `json:"output"`
}

type StageResult struct {
	Name     string          `json:"name"`
	Options  json.RawMessage `json:"options"`
	Success  bool            `json:"success"`
	Output   string          `json:"output"`
	Metadata StageMetadata   `json:"metadata"`
}

// StageMetadata specify the metadata of a given stage-type.
type StageMetadata interface {
	isStageMetadata()
}

type rawStageResult struct {
	Name     string          `json:"name"`
	Options  json.RawMessage `json:"options"`
	Success  bool            `json:"success"`
	Output   string          `json:"output"`
	Metadata json.RawMessage `json:"metadata"`
}

type buildResult struct {
	Stages  []StageResult `json:"stages"`
	TreeID  string        `json:"tree_id"`
	Success bool          `json:"success"`
}

type Result struct {
	TreeID    string              `json:"tree_id"`
	OutputID  string              `json:"output_id"`
	Build     *buildResult        `json:"build"`
	Stages    []StageResult       `json:"stages"`
	Assembler *rawAssemblerResult `json:"assembler"`
	Success   bool                `json:"success"`
}

func (result *StageResult) UnmarshalJSON(data []byte) error {
	var rawStageResult rawStageResult
	err := json.Unmarshal(data, &rawStageResult)
	if err != nil {
		return err
	}
	var metadata StageMetadata
	switch rawStageResult.Name {
	case "org.osbuild.rpm":
		metadata = new(RPMStageMetadata)
		err = json.Unmarshal(rawStageResult.Metadata, metadata)
		if err != nil {
			return err
		}
	default:
		metadata = nil
	}

	result.Name = rawStageResult.Name
	result.Options = rawStageResult.Options
	result.Success = rawStageResult.Success
	result.Output = rawStageResult.Output
	result.Metadata = metadata

	return nil
}

func (cr *Result) Write(writer io.Writer) error {
	if cr.Build == nil && len(cr.Stages) == 0 && cr.Assembler == nil {
		fmt.Fprintf(writer, "The compose result is empty.\n")
	}

	if cr.Build != nil {
		fmt.Fprintf(writer, "Build pipeline:\n")

		for _, stage := range cr.Build.Stages {
			fmt.Fprintf(writer, "Stage %s\n", stage.Name)
			enc := json.NewEncoder(writer)
			enc.SetIndent("", "  ")
			err := enc.Encode(stage.Options)
			if err != nil {
				return err
			}
			fmt.Fprintf(writer, "\nOutput:\n%s\n", stage.Output)
		}
	}

	if len(cr.Stages) > 0 {
		fmt.Fprintf(writer, "Stages:\n")
		for _, stage := range cr.Stages {
			fmt.Fprintf(writer, "Stage: %s\n", stage.Name)
			enc := json.NewEncoder(writer)
			enc.SetIndent("", "  ")
			err := enc.Encode(stage.Options)
			if err != nil {
				return err
			}
			fmt.Fprintf(writer, "\nOutput:\n%s\n", stage.Output)
		}
	}

	if cr.Assembler != nil {
		fmt.Fprintf(writer, "Assembler %s:\n", cr.Assembler.Name)
		enc := json.NewEncoder(writer)
		enc.SetIndent("", "  ")
		err := enc.Encode(cr.Assembler.Options)
		if err != nil {
			return err
		}
		fmt.Fprintf(writer, "\nOutput:\n%s\n", cr.Assembler.Output)
	}

	return nil
}

func (cr *Result) UnmarshalJSON(data []byte) error {
	// NOTE(akoutsou) 1to2t: result format depends on the osbuild version; this
	// unmarshaller tries to read both old and new versions
	type resultAlias Result
	crv1 := new(resultAlias)

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&crv1); err == nil {
		*cr = Result(*crv1)
		return nil
	}

	// try osbuild2 result object
	crv2 := new(osbuild2.Result)

	// NOTE: Using plain (non-strict) Unmarshal here.  The format of the new
	// osbuild output schema is not yet fixed and is likely to change, so
	// disallowing unknown fields will likely cause failures in the near future.
	if err := json.Unmarshal(data, &crv2); err != nil {
		return err
	}
	cr.fromV2(crv2)
	return nil
}

// Convert new OSBuild v2 format result into a v1 by copying the most useful
// values:
// - Compose success status
// - Output of Stages (Log) as flattened list of v1 StageResults
func (cr *Result) fromV2(crv2 *osbuild2.Result) {
	cr.Success = crv2.Success
	// Empty build and assembler results for new types of jobs
	cr.Build = new(buildResult)
	cr.Assembler = new(rawAssemblerResult)

	// convert all stages logs from all pipelines into v1 StageResult objects
	for pname, stages := range crv2.Log {
		for idx, stage := range stages {
			stageResult := StageResult{
				// Create uniquely identifiable name for the stage:
				// <pipeline name>:<stage index>-<stage type>
				Name:    fmt.Sprintf("%s:%d-%s", pname, idx, stage.Type),
				Success: stage.Success,
				Output:  stage.Output,
			}
			cr.Stages = append(cr.Stages, stageResult)
		}
	}
}
