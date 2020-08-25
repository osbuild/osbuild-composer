package osbuild

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteFull(t *testing.T) {

	const testOptions = `{"msg": "test"}`

	testStage := stage{
		Name:    "testStage",
		Options: []byte(testOptions),
		Success: true,
		Output:  "Finished",
	}

	testBuild := build{
		Stages:  []stage{testStage},
		TreeID:  "treeID",
		Success: true,
	}

	testAssembler := assembler{
		Name:    "testAssembler",
		Options: []byte(testOptions),
		Success: true,
		Output:  "Done",
	}

	testComposeResult := Result{
		TreeID:    "TreeID",
		OutputID:  "OutputID",
		Build:     &testBuild,
		Stages:    []stage{testStage},
		Assembler: &testAssembler,
		Success:   true,
	}

	var b bytes.Buffer
	assert.NoError(t, testComposeResult.Write(&b))
	expectedMessage :=
		`Build pipeline:
Stage testStage
{
  "msg": "test"
}

Output:
Finished
Stages:
Stage: testStage
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
