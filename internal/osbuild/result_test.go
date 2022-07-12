package osbuild

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
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

func TestWrite(t *testing.T) {
	assert := assert.New(t)
	result := Result{
		Type:    "result",
		Success: true,
		Error:   []byte{},
		Log: map[string]PipelineResult{
			"build": {
				StageResult{
					ID:     "7de66f934df730534ea120a819480fb94913a5f448aa2fe6827c49d884cb9bd1",
					Type:   "org.osbuild.rpm",
					Output: "<rpm stage output>",
				},
				StageResult{
					ID:     "5fca77a102097ff631d7e125a838a42235c7bfbdbd45fd872a61e3f0805125c5",
					Type:   "org.osbuild.selinux",
					Output: "<selinux stage output>",
				},
			},
			"os": {
				StageResult{
					ID:     "some ID",
					Type:   "org.osbuild.rpm",
					Output: "<os rpm stage output>",
				},
			},
			"final-pipeline": {
				StageResult{
					ID:     "assembler ID",
					Type:   "org.osbuild.qcow2",
					Output: "assmelber the image",
				},
			},
		},
		Metadata: map[string]PipelineMetadata{
			"build": map[string]StageMetadata{
				"org.osbuild.rpm": RPMStageMetadata{
					Packages: []RPMPackageMetadata{
						{
							Name:    "fake-package",
							Version: "1",
							Release: "2",
							Arch:    "x86_64",
						},
					},
				},
			},
			"os": map[string]StageMetadata{
				"org.osbuild.rpm": RPMStageMetadata{
					Packages: []RPMPackageMetadata{
						{
							Name:    "vim-minimal",
							Version: "8.0.1763",
							Release: "15.el8",
							Epoch:   common.StringToPtr("2"),
							Arch:    "x86_64",
							SigMD5:  "4b3ddc56cbb1be95e0973b4a98047820",
							SigPGP:  "8902150305005ed77b28199e2f91fd431d510108a4141000995156bc9f610ad386ee49c42ab31c864fc605cae26592ba58f973fe97b54ea12b42c8e7ee2d716162714fe815de63b60cadb7400a0c71aa56dd3b0af656c6ea413eaaada53374e2e910e556d90e4d157a5b41a6540e355a0176fb3879bf17d90533d1aa3b3d23f06a99a42ad80f17498af2c321193b7be5a504f5dc759d6787a180f9fb3c1903be75f448429537eb0abeb96bb2e73cdc5fe91465c3d54154f6717ffd0a1b42a178e5093500d475639ef60ee483a1ec0d3148d23e0c2ab7bde7c68e5dfdd1103f8e9da7d53ec637c057bc1496d0504fe92760942f9f6de7382fbdef481489c7f6f943bf7fb8c8aadb6484569a6a8f074db78f84579dbaccc86c1eb49379b47033a9eca2577df00d60b353b08bc3850d852365792f194dd8b2b9ba4a1ad5c103afd4db853382520a64ecc362339f3642f4f1ad4e52d8f67b2e731b8d10cef29cb3ed05837245bfca37335f3760f3fb64cbf7acae7e18916a3d4272b0d1589320ab963123649eb9722c8c0e444952900caf39caa371fa77bec8a0e4b010f370eab3d4fe5653a38d88a5a4a415a89f917a31da856a4616ae07ce5749d90ac84bb9189263b162e0cf54ba58a8012d64c89196abae9113e0cda60b4e86879e23d8693691a234784ad3e161733798a0aa41416c045feeb8e2f5859a8a64272298da3d2c1ece675ee802fe8cb273e0b3c1b0f00960d3da09adbdbc531e",
						},
					},
				},
			},
		}}

	var b bytes.Buffer
	assert.NoError(result.Write(&b))
	expectedOutput :=
		`Pipeline build
Stage org.osbuild.rpm
Output:
<rpm stage output>
Metadata:
{
  "packages": [
    {
      "name": "fake-package",
      "version": "1",
      "release": "2",
      "epoch": null,
      "arch": "x86_64",
      "sigmd5": "",
      "sigpgp": "",
      "siggpg": ""
    }
  ]
}

Stage org.osbuild.selinux
Output:
<selinux stage output>

Pipeline final-pipeline
Stage org.osbuild.qcow2
Output:
assmelber the image
Pipeline os
Stage org.osbuild.rpm
Output:
<os rpm stage output>
Metadata:
{
  "packages": [
    {
      "name": "vim-minimal",
      "version": "8.0.1763",
      "release": "15.el8",
      "epoch": "2",
      "arch": "x86_64",
      "sigmd5": "4b3ddc56cbb1be95e0973b4a98047820",
      "sigpgp": "8902150305005ed77b28199e2f91fd431d510108a4141000995156bc9f610ad386ee49c42ab31c864fc605cae26592ba58f973fe97b54ea12b42c8e7ee2d716162714fe815de63b60cadb7400a0c71aa56dd3b0af656c6ea413eaaada53374e2e910e556d90e4d157a5b41a6540e355a0176fb3879bf17d90533d1aa3b3d23f06a99a42ad80f17498af2c321193b7be5a504f5dc759d6787a180f9fb3c1903be75f448429537eb0abeb96bb2e73cdc5fe91465c3d54154f6717ffd0a1b42a178e5093500d475639ef60ee483a1ec0d3148d23e0c2ab7bde7c68e5dfdd1103f8e9da7d53ec637c057bc1496d0504fe92760942f9f6de7382fbdef481489c7f6f943bf7fb8c8aadb6484569a6a8f074db78f84579dbaccc86c1eb49379b47033a9eca2577df00d60b353b08bc3850d852365792f194dd8b2b9ba4a1ad5c103afd4db853382520a64ecc362339f3642f4f1ad4e52d8f67b2e731b8d10cef29cb3ed05837245bfca37335f3760f3fb64cbf7acae7e18916a3d4272b0d1589320ab963123649eb9722c8c0e444952900caf39caa371fa77bec8a0e4b010f370eab3d4fe5653a38d88a5a4a415a89f917a31da856a4616ae07ce5749d90ac84bb9189263b162e0cf54ba58a8012d64c89196abae9113e0cda60b4e86879e23d8693691a234784ad3e161733798a0aa41416c045feeb8e2f5859a8a64272298da3d2c1ece675ee802fe8cb273e0b3c1b0f00960d3da09adbdbc531e",
      "siggpg": ""
    }
  ]
}

`
	assert.Equal(expectedOutput, b.String())
}

func TestWriteEmpty(t *testing.T) {
	assert := assert.New(t)
	var b bytes.Buffer

	var testNilResult *Result
	assert.NoError(testNilResult.Write(&b))
	assert.Equal("The compose result is empty.\n", b.String())

	b.Reset()
	result := Result{}
	assert.NoError(result.Write(&b))
	assert.Equal("The compose result is empty.\n", b.String())
}
