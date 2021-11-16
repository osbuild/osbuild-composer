package osbuild1

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/osbuild2"
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
	if cr == nil || (cr.Build == nil && len(cr.Stages) == 0 && cr.Assembler == nil) {
		fmt.Fprintf(writer, "The compose result is empty.\n")
		return nil
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

// isV2Result returns true if data contains a json-encoded osbuild result
// in version 2 schema.
//
// It detects the schema version by checking if the decoded json contains
// a "type" field at the top-level.
//
// error is non-nil when data isn't a json-encoded object.
func isV2Result(data []byte) (bool, error) {
	var v2ResultStub struct {
		Type string `json:"type"`
	}

	err := json.Unmarshal(data, &v2ResultStub)
	if err != nil {
		return false, err
	}

	return v2ResultStub.Type != "", nil
}

// UnmarshalJSON decodes json-encoded data into a Result struct.
//
// Note that this function is smart and if a result from manifest v2 is given,
// it detects it and converts it to a result like it would be returned for
// manifest v1. This conversion is always lossy.
//
// TODO: We might want to get rid of the smart behaviour and make this method
//       dumb again.
func (cr *Result) UnmarshalJSON(data []byte) error {
	// detect if the input is v2 result
	v2Result, err := isV2Result(data)
	if err != nil {
		return err
	}
	if v2Result {
		// do the best-effort conversion from v2
		var crv2 osbuild2.Result

		// NOTE: Using plain (non-strict) Unmarshal here.  The format of the new
		// osbuild output schema is not yet fixed and is likely to change, so
		// disallowing unknown fields will likely cause failures in the near future.
		if err := json.Unmarshal(data, &crv2); err != nil {
			return err
		}
		cr.fromV2(crv2)
		return nil
	}

	// otherwise, unmarshal using a type alias to prevent recursive calls
	// of this method.
	type resultAlias Result
	var crv1 resultAlias
	err = json.Unmarshal(data, &crv1)
	if err != nil {
		return err
	}

	*cr = Result(crv1)
	return nil
}

// Convert new OSBuild v2 format result into a v1 by copying the most useful
// values:
// - Compose success status
// - Output of Stages (Log) as flattened list of v1 StageResults
func (cr *Result) fromV2(crv2 osbuild2.Result) {
	cr.Success = crv2.Success
	// Empty build and assembler results for new types of jobs
	cr.Build = new(buildResult)
	cr.Assembler = new(StageResult)

	// crv2.Log contains a map of pipelines. Unfortunately, Go doesn't
	// preserve the order of keys in a map. See:
	// https://github.com/golang/go/issues/27179
	//
	// I think it makes sense for this function to always return
	// a well-defined output, therefore we need to invent an ordering
	// for pipeline results. Otherwise, the ordering is basically random.
	//
	// The following lines convert the map of pipeline results to an array
	// of pipeline results. In the last step, the array is sorted by
	// the pipeline name. This isn't ideal but at least it's predictable.
	//
	// See: https://github.com/osbuild/osbuild/issues/619
	type pipelineResult struct {
		pipelineName string
		stageResults []osbuild2.StageResult
	}

	var pipelineResults []pipelineResult

	for pname, stageResults := range crv2.Log {
		pipelineResults = append(pipelineResults, pipelineResult{pipelineName: pname, stageResults: stageResults})
	}

	// Sort the pipelineResult array by the pipeline name to ensure a stable order.
	sort.Slice(pipelineResults, func(i, j int) bool {
		return pipelineResults[i].pipelineName < pipelineResults[j].pipelineName
	})

	v2metadata := crv2.Metadata
	// convert all stages logs from all pipelines into v1 StageResult objects
	for _, pr := range pipelineResults {
		pipelineMetadata := v2metadata[pr.pipelineName]
		for idx, stage := range pr.stageResults {
			stageMetadataV2 := pipelineMetadata[stage.Type]
			stageMetadata, _ := convertStageMetadata(stageMetadataV2, stage.Type)
			stageResult := StageResult{
				// Create uniquely identifiable name for the stage:
				// <pipeline name>:<stage index>-<stage type>
				Name:     fmt.Sprintf("%s:%d-%s", pr.pipelineName, idx, stage.Type),
				Success:  stage.Success,
				Output:   stage.Output,
				Metadata: stageMetadata,
			}
			cr.Stages = append(cr.Stages, stageResult)
		}
	}
}

func convertStageMetadata(v2md osbuild2.StageMetadata, stageType string) (StageMetadata, error) {
	if v2md == nil {
		return nil, nil
	}
	switch metadata := v2md.(type) {
	case *osbuild2.RPMStageMetadata:
		packages := make([]RPMPackageMetadata, len(metadata.Packages))
		for idx, pkg := range metadata.Packages {
			packages[idx] = RPMPackageMetadata{
				Name:    pkg.Name,
				Version: pkg.Version,
				Release: pkg.Release,
				Epoch:   pkg.Epoch,
				Arch:    pkg.Arch,
				SigMD5:  pkg.SigMD5,
				SigPGP:  pkg.SigPGP,
				SigGPG:  pkg.SigGPG,
			}
		}
		return RPMStageMetadata{Packages: packages}, nil
	case *osbuild2.OSTreeCommitStageMetadata:
		v2compose := metadata.Compose
		commitMetadata := OSTreeCommitStageMetadata{
			Compose: OSTreeCommitStageMetadataCompose{
				Ref:                       v2compose.Ref,
				OSTreeNMetadataTotal:      v2compose.OSTreeNMetadataTotal,
				OSTreeNMetadataWritten:    v2compose.OSTreeNMetadataWritten,
				OSTreeNContentTotal:       v2compose.OSTreeNContentTotal,
				OSTreeNContentWritten:     v2compose.OSTreeNContentWritten,
				OSTreeNCacheHits:          v2compose.OSTreeNCacheHits,
				OSTreeContentBytesWritten: v2compose.OSTreeContentBytesWritten,
				OSTreeCommit:              v2compose.OSTreeCommit,
				OSTreeContentChecksum:     v2compose.OSTreeContentChecksum,
				OSTreeTimestamp:           v2compose.OSTreeTimestamp,
				RPMOSTreeInputHash:        v2compose.RPMOSTreeInputHash,
			},
		}
		return commitMetadata, nil
	}

	// any other type, return raw
	raw, err := json.Marshal(v2md)
	if err != nil {
		return nil, err
	}
	return RawStageMetadata(raw), nil
}
