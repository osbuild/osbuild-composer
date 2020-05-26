// This package contains tests related to dnf-json and rpmmd package.

// +build integration

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/test"
)

func TestFetchChecksum(t *testing.T) {
	dir, err := test.SetUpTemporaryRepository()
	defer func(dir string) {
		err := test.TearDownTemporaryRepository(dir)
		assert.Nil(t, err, "Failed to clean up temporary repository.")
	}(dir)
	assert.Nilf(t, err, "Failed to set up temporary repository: %v", err)

	repoCfg := rpmmd.RepoConfig{
		Name:      "repo",
		BaseURL:   fmt.Sprintf("file://%s", dir),
		IgnoreSSL: true,
	}
	rpmMetadata := rpmmd.NewRPMMD(path.Join(dir, "rpmmd"))
	_, c, err := rpmMetadata.FetchMetadata([]rpmmd.RepoConfig{repoCfg}, "platform:f31", "x86_64")
	assert.Nilf(t, err, "Failed to fetch checksum: %v", err)
	assert.NotEqual(t, "", c["repo"], "The checksum is empty")
}

// This test loads all the repositories available in /repositories directory
// and tries to run depsolve for each architecture. With N architectures available
// this should run cross-arch dependency solving N-1 times.
func TestCrossArchDepsolve(t *testing.T) {
	// Load repositories from the definition we provide in the RPM package
	repoDir := "/usr/share/osbuild-composer"

	// NOTE: we can add RHEL, but don't make it hard requirement because it will fail outside of VPN
	for _, distroStruct := range []distro.Distro{fedora31.New(), fedora32.New()} {
		t.Run(distroStruct.Name(), func(t *testing.T) {

			// Run tests in parallel to speed up run times.
			t.Parallel()

			// Set up temporary directory for rpm/dnf cache
			dir, err := ioutil.TempDir("/tmp", "rpmmd-test-")
			require.Nilf(t, err, "Failed to create tmp dir for depsolve test: %v", err)
			defer os.RemoveAll(dir)
			rpm := rpmmd.NewRPMMD(dir)

			repos, err := rpmmd.LoadRepositories([]string{repoDir}, distroStruct.Name())
			require.NoErrorf(t, err, "Failed to LoadRepositories %v", distroStruct.Name())

			for _, archStr := range distroStruct.ListArches() {
				t.Run(archStr, func(t *testing.T) {
					arch, err := distroStruct.GetArch(archStr)
					require.NoError(t, err)

					for _, imgTypeStr := range arch.ListImageTypes() {
						t.Run(imgTypeStr, func(t *testing.T) {
							imgType, err := arch.GetImageType(imgTypeStr)
							require.NoError(t, err)

							buildPackages := imgType.BuildPackages()
							_, _, err = rpm.Depsolve(buildPackages, []string{}, repos[archStr], distroStruct.ModulePlatformID(), archStr)
							assert.NoError(t, err)

							basePackagesInclude, basePackagesExclude := imgType.BasePackages()
							_, _, err = rpm.Depsolve(basePackagesInclude, basePackagesExclude, repos[archStr], distroStruct.ModulePlatformID(), archStr)
							assert.NoError(t, err)
						})
					}
				})
			}
		})
	}
}
