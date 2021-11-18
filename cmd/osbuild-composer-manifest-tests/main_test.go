// +build integration

package main

import (
	"flag"
	"os"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/stretchr/testify/require"
)

var manifestsPath string
var dnfJsonPath string
var testCaseGlob string

func init() {
	flag.StringVar(&manifestsPath, "manifests-path", "/usr/share/tests/osbuild-composer/manifests", "path to a directory with *.json files containing image test cases")
	flag.StringVar(&dnfJsonPath, "dnf-json-path", "/usr/libexec/osbuild-composer/dnf-json", "path to the 'dnf-json' executable")
	flag.StringVar(&testCaseGlob, "test-case-glob", "*", "glob pattern to select image test cases to verify")
}

func TestManifests(t *testing.T) {
	cacheDirPath := "/var/tmp/osbuild-composer-manifest-tests/rpmmd"
	err := os.MkdirAll(cacheDirPath, 0755)
	require.Nilf(t, err, "failed to create RPMMD cache directory %q", cacheDirPath)

	distro_test_common.TestDistro_Manifest(
		t,
		manifestsPath,
		testCaseGlob,
		distroregistry.NewDefault(),
		true,
		cacheDirPath,
		dnfJsonPath,
	)
}
