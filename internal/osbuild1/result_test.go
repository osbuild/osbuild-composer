package osbuild1

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshal(t *testing.T) {
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
					"sigmd5": "84fc907a5047aeebaf8da1642925a417",
					"sigpgp": null,
					"siggpg": "883f0305005f2310139ec3e4c0f7e257e611023e11009f639c5fe64abaa76224dab3a9f70c2714a84c63bd009d1cc184fb4b428dfcd7c3556f4a5f860cc0187740"
				  },
				  {
					"name": "whois-nls",
					"version": "5.5.6",
					"release": "1.fc32",
					"epoch": null,
					"arch": "noarch",
					"sigmd5": "f868cd02046630c8ce3a9c48820e2437",
					"sigpgp": "89023304000108001d162104963a2beb02009608fe67ea4249fd77499570ff3105025f5a272b000a091049fd77499570ff31ccdb0ffe38b95a55ebf3c021526b3cd4f2358c7e23f7767d1f5ce4b7cccef7b33653c6a96a23022313a818fbaf7abeb41837910f0d3ac15664e02838d5939d38ff459aa0076e248728a032d3ae09ddfaec955f941601081a2e3f9bbd49586fd65c1bc1b31685aeb0405687d1791471eab7359ccf00d5584ddef680e99ebc8a4846316391b9baa68ac8ed8ad696ee16fd625d847f8edd92517df3ea6920a46b77b4f119715a0f619f38835d25e0bd0eb5cfad08cd9c796eace6a2b28f4d3dee552e6068255d9748dc2a1906c951e0ba8aed9922ab24e1f659413a06083f8a0bfea56cfff14bddef23bced449f36bcd369da72f90ddf0512e7b0801ba5a0c8eaa8eb0582c630815e992192042cfb0a7c7239f76219197c2fdf18b6553260c105280806d4f037d7b04bdf3da9fd7e9a207db5c71f7e548f4288928f047c989c4cb9cbb8088eec7bd2fa5c252e693f51a3cfc660f666af6a255a5ca0fd2216d5ccd66cbd9c11afa61067d7f615ec8d0dc0c879b5fe633d8c9443f97285da597e4da8a3993af36f0be06acfa9b8058ec70bbc78b876e4c6c5d2108fb05c15a74ba48a3d7ded697cbc1748c228d77d1e0794a41fd5240fa67c3ed745fe47555a47c3d6163d8ce95fd6c2d0d6fa48f8e5b411e571e442109b1cb200d9a8117ee08bfe645f96aca34f7b7559622bbab75143dcad59f126ae0d319e6668ebba417e725638c4febf2e",
					"siggpg": null
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
	assert.Empty(t, package1.SigPGP)
	assert.Equal(t, package1.SigGPG, "883f0305005f2310139ec3e4c0f7e257e611023e11009f639c5fe64abaa76224dab3a9f70c2714a84c63bd009d1cc184fb4b428dfcd7c3556f4a5f860cc0187740")

	package2 := metadata.Packages[1]
	assert.Equal(t, package2.SigPGP, "89023304000108001d162104963a2beb02009608fe67ea4249fd77499570ff3105025f5a272b000a091049fd77499570ff31ccdb0ffe38b95a55ebf3c021526b3cd4f2358c7e23f7767d1f5ce4b7cccef7b33653c6a96a23022313a818fbaf7abeb41837910f0d3ac15664e02838d5939d38ff459aa0076e248728a032d3ae09ddfaec955f941601081a2e3f9bbd49586fd65c1bc1b31685aeb0405687d1791471eab7359ccf00d5584ddef680e99ebc8a4846316391b9baa68ac8ed8ad696ee16fd625d847f8edd92517df3ea6920a46b77b4f119715a0f619f38835d25e0bd0eb5cfad08cd9c796eace6a2b28f4d3dee552e6068255d9748dc2a1906c951e0ba8aed9922ab24e1f659413a06083f8a0bfea56cfff14bddef23bced449f36bcd369da72f90ddf0512e7b0801ba5a0c8eaa8eb0582c630815e992192042cfb0a7c7239f76219197c2fdf18b6553260c105280806d4f037d7b04bdf3da9fd7e9a207db5c71f7e548f4288928f047c989c4cb9cbb8088eec7bd2fa5c252e693f51a3cfc660f666af6a255a5ca0fd2216d5ccd66cbd9c11afa61067d7f615ec8d0dc0c879b5fe633d8c9443f97285da597e4da8a3993af36f0be06acfa9b8058ec70bbc78b876e4c6c5d2108fb05c15a74ba48a3d7ded697cbc1748c228d77d1e0794a41fd5240fa67c3ed745fe47555a47c3d6163d8ce95fd6c2d0d6fa48f8e5b411e571e442109b1cb200d9a8117ee08bfe645f96aca34f7b7559622bbab75143dcad59f126ae0d319e6668ebba417e725638c4febf2e")
	assert.Empty(t, package2.SigGPG)
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
