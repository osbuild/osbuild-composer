package main

import (
	"encoding/json"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_osbuildStagesToRPMs(t *testing.T) {
	raw := `
[
  {
    "name": "org.osbuild.fix-bls",
    "options": {},
    "success": true,
    "output": "shortened",
    "metadata": null
  },
  {
    "name": "org.osbuild.rpm",
    "options": {
      "gpgkeys": [
        "shortened"
      ],
      "packages": [
        {
          "checksum": "sha256:342bdf0143d9145f8846e1b5c3401685e0d1274b66df39ac8cbfb78013313861"
        },
        {
          "checksum": "sha256:fd2a2dd726d855f877296227fb351883d647df28b1b0085f525d87df622d49e4"
        }
      ]
    },
    "success": true,
    "output": "shortened",
    "metadata": {
      "packages": [
        {
          "name": "python3-pyserial",
          "version": "3.4",
          "release": "7.fc32",
          "epoch": null,
          "arch": "noarch",
          "sigmd5": "378cb32f9f850b275ac4e04d21e8144b"
        },
        {
          "name": "libgcc",
          "version": "10.0.1",
          "release": "0.11.fc32",
          "epoch": null,
          "arch": "x86_64",
          "sigmd5": "84fc907a5047aeebaf8da1642925a417"
        }
      ]
    }
  }
]
`
	var stageResults []osbuild.StageResult
	err := json.Unmarshal([]byte(raw), &stageResults)
	require.NoError(t, err)

	rpms := osbuildStagesToRPMs(stageResults)

	require.Len(t, rpms, 2)

	require.Equal(t, koji.RPM{
		Type:      "rpm",
		Name:      "libgcc",
		Version:   "10.0.1",
		Release:   "0.11.fc32",
		Epoch:     nil,
		Arch:      "x86_64",
		Sigmd5:    "84fc907a5047aeebaf8da1642925a417",
		Signature: nil,
	}, rpms[1])
}
