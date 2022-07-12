// a different package is used to prevent import cycles between `rpmmd` and `osbuild`
package rpmmd_test

import (
	"sort"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/stretchr/testify/assert"
)

func TestRPMDeduplication(t *testing.T) {
	assert := assert.New(t)
	// start with metadata, that includes duplicates, convert, then deduplicate
	metadata := osbuild.PipelineMetadata{
		"1": &osbuild.RPMStageMetadata{
			Packages: []osbuild.RPMPackageMetadata{
				// python38 twice
				{
					Name:    "python38",
					Version: "3.8.8",
					Release: "4.module+el8.5.0+12205+a865257a",
					Epoch:   nil,
					Arch:    "x86_64",
					SigMD5:  "-",
					SigPGP:  "-",
					SigGPG:  "-",
				},
				{
					Name:    "python38",
					Version: "3.8.8",
					Release: "4.module+el8.5.0+12205+a865257a",
					Epoch:   nil,
					Arch:    "x86_64",
					SigMD5:  "-",
					SigPGP:  "-",
					SigGPG:  "-",
				},
				// made up package
				{
					Name:    "unique",
					Version: "1.90",
					Release: "10",
					Epoch:   nil,
					Arch:    "aarch64",
					SigMD5:  ".",
					SigPGP:  ".",
					SigGPG:  ".",
				},
				// made up package with epoch
				{
					Name:    "package-with-epoch",
					Version: "0.1",
					Release: "a",
					Epoch:   common.StringToPtr("8"),
					Arch:    "x86_64",
					SigMD5:  "*",
					SigPGP:  "*",
					SigGPG:  "*",
				},
			},
		},
		// separate pipeline
		"2": &osbuild.RPMStageMetadata{
			Packages: []osbuild.RPMPackageMetadata{
				// duplicate package with epoch
				{
					Name:    "vim-minimal",
					Version: "8.0.1763",
					Release: "15.el8",
					Epoch:   common.StringToPtr("2"),
					Arch:    "x86_64",
					SigMD5:  "v",
					SigPGP:  "v",
					SigGPG:  "v",
				},
				{
					Name:    "vim-minimal",
					Version: "8.0.1763",
					Release: "15.el8",
					Epoch:   common.StringToPtr("2"),
					Arch:    "x86_64",
					SigMD5:  "v",
					SigPGP:  "v",
					SigGPG:  "v",
				},
				// package with same name but different version
				{
					Name:    "dupename",
					Version: "1",
					Release: "1.el8",
					Epoch:   nil,
					Arch:    "x86_64",
					SigMD5:  "2",
					SigPGP:  "2",
					SigGPG:  "2",
				},
				{
					Name:    "dupename",
					Version: "2",
					Release: "1.el8",
					Epoch:   nil,
					Arch:    "x86_64",
					SigMD5:  "2",
					SigPGP:  "2",
					SigGPG:  "2",
				},
			},
		},
	}

	testNames := []string{"dupename", "dupename", "package-with-epoch", "python38", "python38", "unique", "vim-minimal", "vim-minimal"}
	testNamesDeduped := []string{"dupename", "dupename", "package-with-epoch", "python38", "unique", "vim-minimal"}

	rpms := osbuild.OSBuildMetadataToRPMs(metadata)

	// basic sanity checks
	assert.Len(rpms, 8)

	sortedNames := func(rpms []rpmmd.RPM) []string {
		names := make([]string, len(rpms))
		for idx, rpm := range rpms {
			names[idx] = rpm.Name
		}

		sort.Strings(names)
		return names
	}

	names := sortedNames(rpms)
	assert.Equal(names, testNames)

	deduped := rpmmd.DeduplicateRPMs(rpms)
	assert.Len(deduped, 6)
	dedupedNames := sortedNames(deduped)
	assert.Equal(dedupedNames, testNamesDeduped)
}
