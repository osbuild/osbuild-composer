package osbuild2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStageResult_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		input   string
		success bool
	}{
		{input: `{}`, success: true},
		{input: `{"success": true}`, success: true},
		{input: `{"success": false}`, success: false},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			var result StageResult
			err := json.Unmarshal([]byte(c.input), &result)
			assert.NoError(t, err)
			assert.Equal(t, c.success, result.Success)
		})
	}
}

func TestUnmarshal(t *testing.T) {
	assert := assert.New(t)
	var result Result
	err := json.Unmarshal([]byte(fullResultRaw), &result)
	assert.NoError(err)

	assert.True(result.Success)

	assert.Equal(len(result.Log), 4)
	for _, plName := range []string{"build", "image", "os", "qcow2"} {
		assert.Contains(result.Log, plName)
		for _, stage := range result.Log[plName] {
			assert.True(stage.Success)
		}
	}
	assert.Equal(len(result.Metadata), 2)

	pipelinePackageCount := map[string]int{
		"build": 254,
		"os":    449,
	}
	for _, plName := range []string{"build", "os"} {
		assert.Contains(result.Metadata, plName)
		for stageType, stageMetadata := range result.Metadata[plName] {
			if stageType == "org.osbuild.rpm" {
				rpmMd, convOk := stageMetadata.(*RPMStageMetadata)
				assert.True(convOk)
				assert.Len(rpmMd.Packages, pipelinePackageCount[plName])
			}
		}
	}
}

func TestUnmarshalV1Success(t *testing.T) {
	var result Result
	err := json.Unmarshal([]byte(v1ResultSuccess), &result)

	assert := assert.New(t)
	assert.NoError(err)

	assert.True(result.Success)

	pipelineStageCount := map[string]int{
		"build":     2,
		"os":        11,
		"assembler": 1,
	}
	assert.Len(result.Log, len(pipelineStageCount))
	for name, stageCount := range pipelineStageCount {
		assert.Contains(result.Log, name)
		assert.Len(result.Log[name], stageCount)
		for _, stage := range result.Log[name] {
			assert.True(stage.Success)
		}
	}

	buildLog := result.Log["build"]
	assert.Equal("org.osbuild.rpm", buildLog[0].Type)

	osLog := result.Log["os"]
	assert.Equal(osLog[0].Type, "org.osbuild.rpm")

	assemblerLog := result.Log["assembler"]
	assert.Equal(assemblerLog[0].Type, "org.osbuild.qemu")

	for _, plName := range []string{"build", "os"} {
		assert.Contains(result.Metadata, plName)
		for stageType, stageMetadata := range result.Metadata[plName] {
			if stageType == "org.osbuild.rpm" {
				rpmMd, convOk := stageMetadata.(*RPMStageMetadata)
				assert.True(convOk)
				assert.Len(rpmMd.Packages, 1)
			}
		}
	}
}

func TestUnmarshalV1Failure(t *testing.T) {
	assert := assert.New(t)
	var result Result
	err := json.Unmarshal([]byte(v1ResultFailure), &result)
	assert.NoError(err)

	assert.False(result.Success)

	pipelineStageCount := map[string]int{
		"build": 2,
		"os":    9,
	}
	assert.Len(result.Log, len(pipelineStageCount))
	for name, stageCount := range pipelineStageCount {
		assert.Contains(result.Log, name)
		assert.Len(result.Log[name], stageCount)
		for idx, stage := range result.Log[name] {
			// lastStage should be false, all else true
			lastStage := (name == "os") && (idx == pipelineStageCount["os"]-1)
			assert.Equal(stage.Success, !lastStage)
		}
	}

	buildLog := result.Log["build"]
	assert.Equal("org.osbuild.rpm", buildLog[0].Type)

	osLog := result.Log["os"]
	assert.False(osLog[8].Success)
	assert.Equal(osLog[0].Type, "org.osbuild.rpm")
}

func TestUnmarshalV2Success(t *testing.T) {
	assert := assert.New(t)
	var result Result
	err := json.Unmarshal([]byte(v2ResultSuccess), &result)
	assert.NoError(err)

	assert.True(result.Success)

	pipelineStageCount := map[string]int{
		"build":          2,
		"ostree-tree":    7,
		"ostree-commit":  2,
		"container-tree": 4,
		"assembler":      1,
	}
	assert.Len(result.Log, len(pipelineStageCount))
	for name, stageCount := range pipelineStageCount {
		assert.Contains(result.Log, name)
		assert.Len(result.Log[name], stageCount)
		for _, stage := range result.Log[name] {
			assert.True(stage.Success)
		}
	}

	// check metadata
	for _, pipeline := range result.Metadata {
		for stageType, stageMetadata := range pipeline {
			if stageType == "org.osbuild.rpm" {
				rpmMd, convOk := stageMetadata.(*RPMStageMetadata)
				assert.True(convOk)
				assert.Greater(len(rpmMd.Packages), 0)
			} else if stageType == "org.osbuild.ostree.commit" {
				commitMd, convOk := stageMetadata.(*OSTreeCommitStageMetadata)
				assert.True(convOk)
				assert.NotEmpty(commitMd.Compose.Ref)
			}
		}
	}
}

func TestUnmarshalV2Failure(t *testing.T) {
	assert := assert.New(t)
	var result Result
	err := json.Unmarshal([]byte(v2ResultFailure), &result)
	assert.NoError(err)

	assert.False(result.Success)

	pipelineStageCount := map[string]int{
		"build":       2,
		"ostree-tree": 5,
	}
	assert.Len(result.Log, len(pipelineStageCount))
	for name, stageCount := range pipelineStageCount {
		assert.Contains(result.Log, name)
		assert.Len(result.Log[name], stageCount)
		for idx, stage := range result.Log[name] {
			// success of last stage in last pipeline should be 'false' and
			// 'true' everywhere else (Success == !lastStage)
			lastStage := (name == "ostree-tree") && (idx == pipelineStageCount["ostree-tree"]-1)
			assert.Equal(stage.Success, !lastStage)
		}
	}
}
