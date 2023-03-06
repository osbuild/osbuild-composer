package osbuild

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockManifest struct {
	Pipelines []MockPipeline `json:"pipelines"`
}

type MockPipeline struct {
	Name string `json:"name"`
	Stages []MockStage `json:"stages"`
}

type MockStage struct {
	Type string `json:"type"`
	Options interface{} `json:"options"`
}

func makeInvalidManifest() []byte {
	return []byte{0}
}

func makeManifestWithNoOsStage() []byte {
	m, _ := json.Marshal(&MockManifest {
		Pipelines: []MockPipeline {
			{
				Name: "not-os",
				Stages: []MockStage{},
			},
		},
	})

	return m
}

func makeManifestWithOsStage() []byte {
	m, _ := json.Marshal(&MockManifest {
		Pipelines: []MockPipeline {
			{
				Name: "os",
				Stages: []MockStage{},
			},
		},
	})

	return m
}

func makeManifestWithOsStageAndRhsmStage() []byte {
	opts := make(map[string]map[string]interface{})
	opts["facts"] = make(map[string]interface{})

	m, _ := json.Marshal(&MockManifest {
		Pipelines: []MockPipeline {
			{
				Name: "os",
				Stages: []MockStage{
					{
						Type: "org.osbuild.rhsm.facts",
						Options: opts,
					},
				},
			},
		},
	})

	return m
}

func TestInjection_InvalidManifest(t *testing.T) {
	buf, err := injectJobDetailsStageIntoManifest(makeInvalidManifest(), "FOO")

	assert.Nil(t, buf)
	assert.NotNil(t, err)
}


func TestInjection_NotOSStageManifest(t *testing.T) {
	buf, err := injectJobDetailsStageIntoManifest(makeManifestWithNoOsStage(), "FOO")

	assert.Nil(t, buf)
	assert.NotNil(t, err)
}


func TestInjection_ManifestWithOsStage(t *testing.T) {
	buf, err := injectJobDetailsStageIntoManifest(makeManifestWithOsStage(), "FOO")

	assert.Nil(t, err)
	expected := `
	{
		"pipelines": [
			{
				"name": "os",
				"stages": [
					{"type": "org.osbuild.os-release.image_id", "options": {"image_id": "FOO"}}
				]
			}
		]
	}
	`

	assert.JSONEq(t, string(buf), expected)
}


func TestInjection_ManifestWithOsAndRhsm(t *testing.T) {
	buf, err := injectJobDetailsStageIntoManifest(makeManifestWithOsStageAndRhsmStage(), "FOO")

	assert.Nil(t, err)
	expected := `
	{
		"pipelines": [
			{
				"name": "os",
				"stages": [
					{"type": "org.osbuild.rhsm.facts", "options": {"facts": {"image-builder.osbuild-composer.image_id": "FOO"}}},
					{"type": "org.osbuild.os-release.image_id", "options": {"image_id": "FOO"}}
				]
			}
		]
	}
	`

	assert.JSONEq(t, string(buf), expected)
}
