//go:build integration

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/osbuild/images/pkg/distro/distro_test_common"
	"github.com/osbuild/images/pkg/distroregistry"
	"github.com/stretchr/testify/assert"
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

// TestImageTestCoverage ensures that each defined image type has
// at least one corresponding image test case.
func TestImageTestCoverage(t *testing.T) {
	distroRegistry := distroregistry.NewDefault()
	for _, distroName := range distroRegistry.List() {
		distro := distroRegistry.GetDistro(distroName)
		for _, archName := range distro.ListArches() {
			missingImgTests := []string{}
			arch, err := distro.GetArch(archName)
			require.Nilf(t, err, "failed to get arch %q of distro %q, which was returned in the list of available arches", archName, distroName)
			for _, imageTypeName := range arch.ListImageTypes() {
				imageTypeGlob := fmt.Sprintf(
					"%s/%s-%s-%s*.json",
					manifestsPath,
					strings.ReplaceAll(distroName, "-", "_"),
					strings.ReplaceAll(archName, "-", "_"),
					strings.ReplaceAll(imageTypeName, "-", "_"),
				)

				testCaseFiles, err := filepath.Glob(imageTypeGlob)
				require.Nilf(t, err, "error while globing for image test cases: %v", err)

				if testCaseFiles == nil {
					missingImgTests = append(missingImgTests, imageTypeName)
				}
			}

			assert.Emptyf(t, missingImgTests, "missing image test cases for %q/%q: %v", distroName, archName, missingImgTests)
		}
	}
}
