package osbuild1

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

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

type RawStageMetadata json.RawMessage

func (RawStageMetadata) isStageMetadata() {}

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
	TreeID    string        `json:"tree_id"`
	OutputID  string        `json:"output_id"`
	Build     *buildResult  `json:"build"`
	Stages    []StageResult `json:"stages"`
	Assembler *StageResult  `json:"assembler"`
	Success   bool          `json:"success"`
}

func (result *StageResult) UnmarshalJSON(data []byte) error {
	var rawStageResult rawStageResult
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
