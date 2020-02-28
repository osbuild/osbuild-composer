package common

import (
	"encoding/json"
	"fmt"
	"io"
)

type ComposeResult struct {
	TreeID   string `json:"tree_id"`
	OutputID string `json:"output_id"`
	Build    *struct {
		Stages []struct {
			Name    string          `json:"name"`
			Options json.RawMessage `json:"options"`
			Success bool            `json:"success"`
			Output  string          `json:"output"`
		} `json:"stages"`
		TreeID  string `json:"tree_id"`
		Success bool   `json:"success"`
	} `json:"build"`
	Stages []struct {
		Name    string          `json:"name"`
		Options json.RawMessage `json:"options"`
		Success bool            `json:"success"`
		Output  string          `json:"output"`
	} `json:"stages"`
	Assembler *struct {
		Name    string          `json:"name"`
		Options json.RawMessage `json:"options"`
		Success bool            `json:"success"`
		Output  string          `json:"output"`
	} `json:"assembler"`
	Success bool `json:"success"`
}

func (cr *ComposeResult) Write(writer io.Writer) error {
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
