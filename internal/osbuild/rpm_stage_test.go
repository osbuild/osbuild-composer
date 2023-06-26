package osbuild

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRPMStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.rpm",
		Options: &RPMStageOptions{},
		Inputs:  &RPMStageInputs{},
	}
	actualStage := NewRPMStage(&RPMStageOptions{}, &RPMStageInputs{})
	assert.Equal(t, expectedStage, actualStage)
}

func Test_OSBuildMetadataToRPMs(t *testing.T) {
	raw := `
{
  "org.osbuild.rpm": {
    "packages": [
      {
        "name": "python3-pyserial",
        "version": "3.4",
        "release": "7.fc32",
        "epoch": null,
        "arch": "noarch",
        "sigmd5": "378cb32f9f850b275ac4e04d21e8144b",
        "sigpgp": "89023304000108001d162104963a2beb02009608fe67ea4249fd77499570ff3105025f5a272b000a091049fd77499570ff31ccdb0ffe38b95a55ebf3c021526b3cd4f2358c7e23f7767d1f5ce4b7cccef7b33653c6a96a23022313a818fbaf7abeb41837910f0d3ac15664e02838d5939d38ff459aa0076e248728a032d3ae09ddfaec955f941601081a2e3f9bbd49586fd65c1bc1b31685aeb0405687d1791471eab7359ccf00d5584ddef680e99ebc8a4846316391b9baa68ac8ed8ad696ee16fd625d847f8edd92517df3ea6920a46b77b4f119715a0f619f38835d25e0bd0eb5cfad08cd9c796eace6a2b28f4d3dee552e6068255d9748dc2a1906c951e0ba8aed9922ab24e1f659413a06083f8a0bfea56cfff14bddef23bced449f36bcd369da72f90ddf0512e7b0801ba5a0c8eaa8eb0582c630815e992192042cfb0a7c7239f76219197c2fdf18b6553260c105280806d4f037d7b04bdf3da9fd7e9a207db5c71f7e548f4288928f047c989c4cb9cbb8088eec7bd2fa5c252e693f51a3cfc660f666af6a255a5ca0fd2216d5ccd66cbd9c11afa61067d7f615ec8d0dc0c879b5fe633d8c9443f97285da597e4da8a3993af36f0be06acfa9b8058ec70bbc78b876e4c6c5d2108fb05c15a74ba48a3d7ded697cbc1748c228d77d1e0794a41fd5240fa67c3ed745fe47555a47c3d6163d8ce95fd6c2d0d6fa48f8e5b411e571e442109b1cb200d9a8117ee08bfe645f96aca34f7b7559622bbab75143dcad59f126ae0d319e6668ebba417e725638c4febf2e",
        "siggpg": "883f0305005f2310139ec3e4c0f7e257e611023e11009f639c5fe64abaa76224dab3a9f70c2714a84c63bd009d1cc184fb4b428dfcd7c3556f4a5f860cc0187740"
      },
      {
        "name": "libgcc",
        "version": "10.0.1",
        "release": "0.11.fc32",
        "epoch": null,
        "arch": "x86_64",
        "sigmd5": "84fc907a5047aeebaf8da1642925a417",
        "sigpgp": "89023304000108001d162104963a2beb02009608fe67ea4249fd77499570ff3105025f5a272b000a091049fd77499570ff31ccdb0ffe38b95a55ebf3c021526b3cd4f2358c7e23f7767d1f5ce4b7cccef7b33653c6a96a23022313a818fbaf7abeb41837910f0d3ac15664e02838d5939d38ff459aa0076e248728a032d3ae09ddfaec955f941601081a2e3f9bbd49586fd65c1bc1b31685aeb0405687d1791471eab7359ccf00d5584ddef680e99ebc8a4846316391b9baa68ac8ed8ad696ee16fd625d847f8edd92517df3ea6920a46b77b4f119715a0f619f38835d25e0bd0eb5cfad08cd9c796eace6a2b28f4d3dee552e6068255d9748dc2a1906c951e0ba8aed9922ab24e1f659413a06083f8a0bfea56cfff14bddef23bced449f36bcd369da72f90ddf0512e7b0801ba5a0c8eaa8eb0582c630815e992192042cfb0a7c7239f76219197c2fdf18b6553260c105280806d4f037d7b04bdf3da9fd7e9a207db5c71f7e548f4288928f047c989c4cb9cbb8088eec7bd2fa5c252e693f51a3cfc660f666af6a255a5ca0fd2216d5ccd66cbd9c11afa61067d7f615ec8d0dc0c879b5fe633d8c9443f97285da597e4da8a3993af36f0be06acfa9b8058ec70bbc78b876e4c6c5d2108fb05c15a74ba48a3d7ded697cbc1748c228d77d1e0794a41fd5240fa67c3ed745fe47555a47c3d6163d8ce95fd6c2d0d6fa48f8e5b411e571e442109b1cb200d9a8117ee08bfe645f96aca34f7b7559622bbab75143dcad59f126ae0d319e6668ebba417e725638c4febf2e",
        "siggpg": null
      },
      {
        "name": "libgcc-madeup",
        "version": "10.0.1",
        "release": "0.11.fc32",
        "epoch": null,
        "arch": "x86_64",
        "sigmd5": "84fc907a5047aeebaf8da1642925a418",
        "sigpgp": null,
        "siggpg": null
      }
    ]
  }
}
`
	metadata := new(PipelineMetadata)
	err := json.Unmarshal([]byte(raw), metadata)
	require.NoError(t, err)

	fmt.Printf("Result: %#v", metadata)
	rpms := OSBuildMetadataToRPMs(*metadata)

	require.Len(t, rpms, 3)

	signature1 := "89023304000108001d162104963a2beb02009608fe67ea4249fd77499570ff3105025f5a272b000a091049fd77499570ff31ccdb0ffe38b95a55ebf3c021526b3cd4f2358c7e23f7767d1f5ce4b7cccef7b33653c6a96a23022313a818fbaf7abeb41837910f0d3ac15664e02838d5939d38ff459aa0076e248728a032d3ae09ddfaec955f941601081a2e3f9bbd49586fd65c1bc1b31685aeb0405687d1791471eab7359ccf00d5584ddef680e99ebc8a4846316391b9baa68ac8ed8ad696ee16fd625d847f8edd92517df3ea6920a46b77b4f119715a0f619f38835d25e0bd0eb5cfad08cd9c796eace6a2b28f4d3dee552e6068255d9748dc2a1906c951e0ba8aed9922ab24e1f659413a06083f8a0bfea56cfff14bddef23bced449f36bcd369da72f90ddf0512e7b0801ba5a0c8eaa8eb0582c630815e992192042cfb0a7c7239f76219197c2fdf18b6553260c105280806d4f037d7b04bdf3da9fd7e9a207db5c71f7e548f4288928f047c989c4cb9cbb8088eec7bd2fa5c252e693f51a3cfc660f666af6a255a5ca0fd2216d5ccd66cbd9c11afa61067d7f615ec8d0dc0c879b5fe633d8c9443f97285da597e4da8a3993af36f0be06acfa9b8058ec70bbc78b876e4c6c5d2108fb05c15a74ba48a3d7ded697cbc1748c228d77d1e0794a41fd5240fa67c3ed745fe47555a47c3d6163d8ce95fd6c2d0d6fa48f8e5b411e571e442109b1cb200d9a8117ee08bfe645f96aca34f7b7559622bbab75143dcad59f126ae0d319e6668ebba417e725638c4febf2e"
	require.Equal(t, rpmmd.RPM{
		Type:      "rpm",
		Name:      "libgcc",
		Version:   "10.0.1",
		Release:   "0.11.fc32",
		Epoch:     nil,
		Arch:      "x86_64",
		Sigmd5:    "84fc907a5047aeebaf8da1642925a417",
		Signature: &signature1,
	}, rpms[1])

	// GPG has a priority over PGP
	signature0 := "883f0305005f2310139ec3e4c0f7e257e611023e11009f639c5fe64abaa76224dab3a9f70c2714a84c63bd009d1cc184fb4b428dfcd7c3556f4a5f860cc0187740"
	require.Equal(t, signature0, *rpms[0].Signature)

	// if neither GPG nor PGP is set, the signature is nil
	require.Nil(t, rpms[2].Signature)
}

func TestNewRpmStageSourceFilesInputs(t *testing.T) {

	assert := assert.New(t)
	require := require.New(t)

	pkgSpecs := []rpmmd.PackageSpec{
		{
			Name:           "openssl-libs",
			Epoch:          1,
			Version:        "3.0.1",
			Release:        "5.el9",
			Arch:           "x86_64",
			RemoteLocation: "https://example.com/repo/Packages/openssl-libs-3.0.1-5.el9.x86_64.rpm",
			Checksum:       "sha256:fcf2515ec9115551c99d552da721803ecbca23b7ae5a974309975000e8bef666",
			Secrets:        "",
			CheckGPG:       false,
			IgnoreSSL:      true,
		},
		{
			Name:           "openssl-pkcs11",
			Epoch:          0,
			Version:        "0.4.11",
			Release:        "7.el9",
			Arch:           "x86_64",
			RemoteLocation: "https://example.com/repo/Packages/openssl-pkcs11-0.4.11-7.el9.x86_64.rpm",
			Checksum:       "sha256:4be41142a5fb2b4cd6d812e126838cffa57b7c84e5a79d65f66bb9cf1d2830a3",
			Secrets:        "",
			CheckGPG:       false,
			IgnoreSSL:      true,
		},
		{
			Name:           "p11-kit",
			Epoch:          0,
			Version:        "0.24.1",
			Release:        "2.el9",
			Arch:           "x86_64",
			RemoteLocation: "https://example.com/repo/Packages/p11-kit-0.24.1-2.el9.x86_64.rpm",
			Checksum:       "sha256:da167e41efd19cf25fd1c708b6f123d0203824324b14dd32401d49f2aa0ef0a6",
			Secrets:        "",
			CheckGPG:       false,
			IgnoreSSL:      true,
		},
		{
			Name:           "package-with-sha1-checksum",
			Epoch:          1,
			Version:        "3.4.2.",
			Release:        "10.el9",
			Arch:           "x86_64",
			RemoteLocation: "https://example.com/repo/Packages/package-with-sha1-checksum-4.3.2-10.el9.x86_64.rpm",
			Checksum:       "sha1:6e01b8076a2ab729d564048bf2e3a97c7ac83c13",
			Secrets:        "",
			CheckGPG:       true,
			IgnoreSSL:      true,
		},
		{
			Name:           "package-with-md5-checksum",
			Epoch:          1,
			Version:        "3.4.2.",
			Release:        "5.el9",
			Arch:           "x86_64",
			RemoteLocation: "https://example.com/repo/Packages/package-with-md5-checksum-4.3.2-5.el9.x86_64.rpm",
			Checksum:       "md5:8133f479f38118c5f9facfe2a2d9a071",
			Secrets:        "",
			CheckGPG:       true,
			IgnoreSSL:      true,
		},
	}
	inputs := NewRpmStageSourceFilesInputs(pkgSpecs)

	refsArrayPtr, convOk := inputs.Packages.References.(*FilesInputSourceArrayRef)
	require.True(convOk)
	require.NotNil(refsArrayPtr)

	refsArray := *refsArrayPtr

	for idx := range refsArray {
		refItem := refsArray[idx]
		pkg := pkgSpecs[idx]
		assert.Equal(pkg.Checksum, refItem.ID)

		if pkg.CheckGPG {
			// GPG check enabled: metadata expected
			require.NotNil(refItem.Options)
			require.NotNil(refItem.Options.Metadata)

			md, convOk := refItem.Options.Metadata.(*RPMStageReferenceMetadata)
			require.True(convOk)
			require.NotNil(md)
			assert.Equal(md.CheckGPG, pkg.CheckGPG)
		}
	}
}
