package distro_test

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/stretchr/testify/require"
)

func TestDistro_Manifest(t *testing.T) {

	distro_test_common.TestDistro_Manifest(
		t,
		"../../test/data/manifests/",
		"*",
		distroregistry.NewDefault(),
		false, // This test case does not check for changes in the imageType package sets!
		"",
		"",
	)
}

var (
	v1manifests = []string{
		`{}`,
		`
{
	"sources": {
		"org.osbuild.files": {
			"urls": {}
		}
	},
	"pipeline": {
		"build": {
			"pipeline": {
				"stages": []
			},
			"runner": "org.osbuild.rhel84"
		},
		"stages": [],
		"assembler": {
			"name": "org.osbuild.qemu",
			"options": {}
		}
	}
}`,
	}

	v2manifests = []string{
		`{"version": "2"}`,
		`
{
	"version": "2",
	"pipelines": [
		{
			"name": "build",
			"runner": "org.osbuild.rhel84",
			"stages": []
		}
	],
	"sources": {
		"org.osbuild.curl": {
			"items": {}
		}
	}
}`,
	}
)

func TestDistro_Version(t *testing.T) {
	require := require.New(t)
	expectedVersion := "1"
	for idx, rawManifest := range v1manifests {
		manifest := distro.Manifest(rawManifest)
		detectedVersion, err := manifest.Version()
		require.NoError(err, "Could not detect Manifest version for %d: %v", idx, err)
		require.Equal(expectedVersion, detectedVersion, "in manifest %d", idx)
	}

	expectedVersion = "2"
	for idx, rawManifest := range v2manifests {
		manifest := distro.Manifest(rawManifest)
		detectedVersion, err := manifest.Version()
		require.NoError(err, "Could not detect Manifest version for %d: %v", idx, err)
		require.Equal(expectedVersion, detectedVersion, "in manifest %d", idx)
	}

	{
		manifest := distro.Manifest("")
		_, err := manifest.Version()
		require.Error(err, "Empty manifest did not return an error")
	}

	{
		manifest := distro.Manifest("{")
		_, err := manifest.Version()
		require.Error(err, "Invalid manifest did not return an error")
	}
}
