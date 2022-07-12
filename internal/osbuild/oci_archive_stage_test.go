package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOCIArchiveStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.oci-archive",
		Options: &OCIArchiveStageOptions{},
		Inputs:  &OCIArchiveStageInputs{},
	}
	actualStage := NewOCIArchiveStage(&OCIArchiveStageOptions{}, &OCIArchiveStageInputs{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestOCIArchiveInputs(t *testing.T) {
	exp := `{
		"base": {
			"type": "org.osbuild.oci-archive",
			"origin":"org.osbuild.pipeline",
			"references": ["name:container-tree"]
		},
		"layer.1": {
			"type": "org.osbuild.tree",
			"origin": "org.osbuild.pipeline",
			"references": ["name:container-ostree"]
		},
		"layer.2": {
			"type": "org.osbuild.tree",
			"origin": "org.osbuild.pipeline",
			"references": ["name:container-ostree2"]
		}
	}`
	inputs := new(OCIArchiveStageInputs)
	base := &OCIArchiveStageInput{
		References: []string{
			"name:container-tree",
		},
	}
	base.Type = "org.osbuild.oci-archive"
	base.Origin = "org.osbuild.pipeline"

	layer1 := OCIArchiveStageInput{
		References: []string{
			"name:container-ostree",
		},
	}
	layer1.Type = "org.osbuild.tree"
	layer1.Origin = "org.osbuild.pipeline"
	layer2 := OCIArchiveStageInput{
		References: []string{
			"name:container-ostree2",
		},
	}
	layer2.Type = "org.osbuild.tree"
	layer2.Origin = "org.osbuild.pipeline"

	inputs.Base = base
	inputs.Layers = []OCIArchiveStageInput{layer1, layer2}

	data, err := json.Marshal(inputs)
	assert.NoError(t, err)
	assert.JSONEq(t, exp, string(data))

	inputsRead := new(OCIArchiveStageInputs)
	err = json.Unmarshal([]byte(exp), inputsRead)
	assert.NoError(t, err)
	assert.Equal(t, inputs, inputsRead)
}

func TestOCIArchiveInputsErrors(t *testing.T) {
	noBase := `{
		"layer.10": {
			"type": "org.osbuild.tree",
			"origin": "org.osbuild.pipeline",
			"references": ["name:container-ostree"]
		},
		"layer.2": {
			"type": "org.osbuild.tree",
			"origin": "org.osbuild.pipeline",
			"references": ["name:container-ostree2"]
		}
	}`

	inputsRead := new(OCIArchiveStageInputs)
	assert.Error(t, json.Unmarshal([]byte(noBase), inputsRead))

	invalidKey := `{
		"base": {
			"type": "org.osbuild.oci-archive",
			"origin":"org.osbuild.pipeline",
			"references": ["name:container-tree"]
		},
		"not-a-layer": {
			"type": "org.osbuild.tree",
			"origin": "org.osbuild.pipeline",
			"references": ["name:container-ostree2"]
		}
	}`
	assert.Error(t, json.Unmarshal([]byte(invalidKey), inputsRead))
}
