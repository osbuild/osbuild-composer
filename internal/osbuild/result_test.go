package osbuild

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshall(t *testing.T) {
	resultRaw := `{
		"success": true,
		"build": {
		  "success": true,
		  "stages": [
			{
			  "name": "org.osbuild.rpm",
			  "id": "9eb0a6f6fd6e2995e107f5bcc6aa3b19643b02ec133bdc8a8ac614860b1bbf2d",
			  "success": true,
			  "output": "Building...",
			  "metadata": {
				"packages": [
				  {
					"name": "libgcc",
					"version": "10.0.1",
					"release": "0.11.fc32",
					"epoch": null,
					"arch": "x86_64",
					"sigmd5": "84fc907a5047aeebaf8da1642925a417"
				  },
				  {
					"name": "whois-nls",
					"version": "5.5.6",
					"release": "1.fc32",
					"epoch": null,
					"arch": "noarch",
					"sigmd5": "f868cd02046630c8ce3a9c48820e2437"
				  }
				]
			  }
			}
		  ]
		}
	  }`

	var result Result
	err := json.Unmarshal([]byte(resultRaw), &result)
	assert.NoError(t, err)

	assert.Equal(t, result.Build.Stages[0].Name, "org.osbuild.rpm")
	metadata, ok := result.Build.Stages[0].Metadata.(*RPMStageMetadata)
	assert.True(t, ok)
	package1 := metadata.Packages[0]
	assert.Equal(t, package1.Name, "libgcc")
	assert.Nil(t, package1.Epoch)
	assert.Equal(t, package1.Version, "10.0.1")
	assert.Equal(t, package1.Release, "0.11.fc32")
	assert.Equal(t, package1.Arch, "x86_64")
	assert.Equal(t, package1.SigMD5, "84fc907a5047aeebaf8da1642925a417")
}

func TestWriteFull(t *testing.T) {

	const testOptions = `{"msg": "test"}`

	dnfStage := StageResult{
		Name:    "org.osbuild.rpm",
		Options: []byte(testOptions),
		Success: true,
		Output:  "Finished",
		Metadata: RPMStageMetadata{
			Packages: []RPMPackageMetadata{
				{
					Name:    "foobar",
					Epoch:   nil,
					Version: "1",
					Release: "1",
					Arch:    "noarch",
					SigMD5:  "deadbeef",
				},
			},
		},
	}

	testStage := StageResult{
		Name:    "org.osbuild.test",
		Options: []byte(testOptions),
		Success: true,
		Output:  "Finished",
	}

	testBuild := buildResult{
		Stages:  []StageResult{testStage},
		TreeID:  "treeID",
		Success: true,
	}

	testAssembler := rawAssemblerResult{
		Name:    "testAssembler",
		Options: []byte(testOptions),
		Success: true,
		Output:  "Done",
	}

	testComposeResult := Result{
		TreeID:    "TreeID",
		OutputID:  "OutputID",
		Build:     &testBuild,
		Stages:    []StageResult{dnfStage},
		Assembler: &testAssembler,
		Success:   true,
	}

	var b bytes.Buffer
	assert.NoError(t, testComposeResult.Write(&b))
	expectedMessage :=
		`Build pipeline:
Stage org.osbuild.test
{
  "msg": "test"
}

Output:
Finished
Stages:
Stage: org.osbuild.rpm
{
  "msg": "test"
}

Output:
Finished
Assembler testAssembler:
{
  "msg": "test"
}

Output:
Done
`
	assert.Equal(t, expectedMessage, b.String())
}

func TestWriteEmpty(t *testing.T) {

	testComposeResult := Result{}

	var b bytes.Buffer
	assert.NoError(t, testComposeResult.Write(&b))
	assert.Equal(t, "The compose result is empty.\n", b.String())

}
